#!/usr/bin/env bash
# go-mall 常用命令封装（Windows 下用 Git Bash 运行）
# 用法：./scripts/tasks.sh <命令> [参数]
set -euo pipefail

cd "$(dirname "$0")/.."

# payment 已经合并进 order：下单即待支付，/orders/:no/pay 直接把状态改成已支付
SERVICES=(gateway user product cart order)
EXE=""
case "$(uname -s)" in
  MINGW* | MSYS* | CYGWIN*) EXE=".exe" ;;
esac

usage() {
  cat <<'EOF'
用法: ./scripts/tasks.sh <命令>

  tidy            整理 Go 依赖（会写入 go.sum）
  fmt             格式化代码
  vet             静态检查
  build           编译全部服务到 bin/
  test            运行单元测试
  test-race       运行竞态检测测试
  run <service>   本地启动某个服务 (gateway|user|product|cart|order)
  start-all       编译并在独立窗口里启动全部服务
  stop-all        关闭全部服务进程
  local-db        在本机 MySQL 上建库建账号（不需要 Docker）
  health          检查各服务健康状态
  infra           启动 MySQL + Redis
  infra-down      停止基础设施
  monitoring      启动 Prometheus（可选）
  ps              查看容器状态
  logs [name]     跟踪容器日志，默认 mysql
  clean           清理编译产物
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
    mkdir -p bin
    for s in "${SERVICES[@]}"; do
      echo "==> building $s"
      go build -o "bin/$s$EXE" "./services/$s/cmd/server"
    done
    # 每个服务开一个独立窗口，方便单独看日志；关掉脚本窗口不影响服务
    for s in "${SERVICES[@]}"; do
      cmd.exe //c start "$s" "bin\\$s$EXE"
      sleep 1
    done
    echo
    echo "已启动：gateway(8080) user(8081) product(8082) cart(8083) order(8084)"
    echo "浏览器打开 http://127.0.0.1:8080"
    ;;
  stop-all)
    for s in "${SERVICES[@]}"; do
      taskkill //F //IM "$s$EXE" >/dev/null 2>&1 || true
    done
    echo "已停止全部服务"
    ;;
  health)
    ports=(8080 8081 8082 8083 8084)
    for i in "${!SERVICES[@]}"; do
      s="${SERVICES[$i]}"
      p="${ports[$i]}"
      printf '%-8s :%s  ' "$s" "$p"
      curl -s -m 2 "http://127.0.0.1:$p/healthz" || echo -n "未启动"
      echo
    done
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
  logs)
    need_docker
    docker compose logs -f "${2:-mysql}"
    ;;
  clean)
    rm -rf bin
    ;;
  *)
    usage
    ;;
esac
