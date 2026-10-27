#!/usr/bin/env bash
# go-mall 常用命令封装（Windows 下用 Git Bash 运行）
# 用法：./scripts/tasks.sh <命令> [参数]
set -euo pipefail

cd "$(dirname "$0")/.."

# payment 已经合并进 order：下单即待支付，/orders/:no/pay 直接把状态改成已支付
SERVICES=(gateway user product cart order)
# 和 SERVICES 一一对应，改顺序时两个数组要一起改
PORTS=(8080 8081 8082 8083 8084)

EXE=""
case "$(uname -s)" in
  MINGW* | MSYS* | CYGWIN*) EXE=".exe" ;;
esac

# 按端口查正在监听的进程 PID（Windows netstat）
# 不按进程名查，因为 user.exe 这种名字太通用，容易误杀别的程序
port_pids() {
  netstat -ano 2>/dev/null \
    | awk -v p=":$1" '$2 ~ p"$" && $4 == "LISTENING" {print $5}' \
    | sort -u
}

# 打印所有服务的健康状态
show_health() {
  for i in "${!SERVICES[@]}"; do
    printf '%-8s :%s  ' "${SERVICES[$i]}" "${PORTS[$i]}"
    if curl -s -m 2 "http://127.0.0.1:${PORTS[$i]}/healthz"; then
      :
    else
      printf '未启动'
    fi
    echo
  done
}

usage() {
  cat <<'EOF'
用法: ./scripts/tasks.sh <命令>

  tidy            整理 Go 依赖（会写入 go.sum）
  fmt             格式化代码
  vet             静态检查
  build           编译全部服务到 bin/
  test            运行单元测试
  test-race       运行竞态检测测试
  run <service>   前台启动某个服务，Ctrl+C 停止 (gateway|user|product|cart|order)
  start-all       编译并在后台启动全部服务，日志写到 logs/
  stop-all        停止全部服务（按端口找进程，不会误杀同名的其他程序）
  log <service>   实时查看某个服务的日志
  local-db        在本机 MySQL 上建库建账号（不需要 Docker）
  health          检查各服务健康状态
  infra           启动 MySQL + Redis
  infra-down      停止基础设施
  monitoring      启动 Prometheus（可选）
  ps              查看容器状态
  docker-logs     跟踪容器日志，默认 mysql
  clean           清理编译产物和日志
EOF
}

need_docker() {
  if ! docker info >/dev/null 2>&1; then
    echo "错误：Docker 守护进程未运行，请先启动 Docker Desktop" >&2
    exit 1
  fi
}

case "${1:-help}" in
  tidy)
    go mod tidy
    ;;
  fmt)
    gofmt -l -w .
    ;;
  vet)
    go vet ./...
    ;;
  build)
    mkdir -p bin
    for s in "${SERVICES[@]}"; do
      echo "==> building $s"
      go build -o "bin/$s$EXE" "./services/$s/cmd/server"
    done
    ;;
  test)
    go test ./...
    ;;
  test-race)
    go test -race ./...
    ;;
  run)
    svc="${2:-}"
    if [[ -z "$svc" ]]; then
      echo "请指定服务名: ${SERVICES[*]}" >&2
      exit 1
    fi
    go run "./services/$svc/cmd/server"
    ;;
  start-all)
    mkdir -p bin logs

    for s in "${SERVICES[@]}"; do
      echo "==> 编译 $s"
      go build -o "bin/$s$EXE" "./services/$s/cmd/server"
    done

    # 先检查端口，避免端口被占用时新进程启动失败却没人发现
    for i in "${!SERVICES[@]}"; do
      s="${SERVICES[$i]}"
      p="${PORTS[$i]}"

      if [[ -n "$(port_pids "$p")" ]]; then
        echo "跳过 $s：端口 $p 已被占用（要重启请先 ./scripts/tasks.sh stop-all）"
        continue
      fi

      # nohup + & ：脚本退出后服务继续跑，输出写到 logs/<服务>.log
      nohup "./bin/$s$EXE" > "logs/$s.log" 2>&1 &
      echo "启动 $s (:$p)  日志 logs/$s.log"
      sleep 1
    done

    echo
    sleep 2
    show_health
    echo
    echo "浏览器打开 http://127.0.0.1:8080"
    ;;
  stop-all)
    stopped=0
    for i in "${!SERVICES[@]}"; do
      s="${SERVICES[$i]}"
      p="${PORTS[$i]}"

      pids="$(port_pids "$p")"
      if [[ -z "$pids" ]]; then
        echo "$s (:$p) 未运行"
        continue
      fi

      for pid in $pids; do
        taskkill //F //PID "$pid" >/dev/null 2>&1 || true
      done
      echo "$s (:$p) 已停止"
      stopped=$((stopped + 1))
    done
    echo "共停止 $stopped 个服务"
    ;;
  log)
    s="${2:-gateway}"
    if [[ ! -f "logs/$s.log" ]]; then
      echo "没有 logs/$s.log。先跑 ./scripts/tasks.sh start-all" >&2
      exit 1
    fi
    tail -f "logs/$s.log"
    ;;
  health)
    show_health
    ;;
  local-db)
    ./scripts/local-db-init.sh
    ;;
  infra)
    need_docker
    docker compose up -d mysql redis
    docker compose ps
    ;;
  infra-down)
    need_docker
    docker compose down
    ;;
  monitoring)
    need_docker
    docker compose --profile monitoring up -d prometheus
    ;;
  ps)
    need_docker
    docker compose ps
    ;;
  docker-logs)
    need_docker
    docker compose logs -f "${2:-mysql}"
    ;;
  clean)
    rm -rf bin logs
    ;;
  *)
    usage
    ;;
esac
