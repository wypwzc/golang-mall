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

# 探测某个端口的 healthz 是否返回 ok。
# 必须加 --noproxy '*'：否则系统代理（如 Mihomo/Clash）会劫持 127.0.0.1 的请求，
# 探不通就误判服务未启动。
health_ok() {
  curl --noproxy '*' -s -m 2 "http://127.0.0.1:$1/healthz" 2>/dev/null \
    | grep -q '"status"[[:space:]]*:[[:space:]]*"ok"'
}

# 打印所有服务的健康状态
show_health() {
  for i in "${!SERVICES[@]}"; do
    printf '%-8s :%s  ' "${SERVICES[$i]}" "${PORTS[$i]}"
    if health_ok "${PORTS[$i]}"; then
      echo "ok"
    else
      echo "未启动"
    fi
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
  local-db        在本机 MySQL 上建库建账号
  health          检查各服务健康状态
  clean           清理编译产物和日志
EOF
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

    # ---- 1. 编译：逐个打印进度 ----
    for s in "${SERVICES[@]}"; do
      echo "==> 编译 $s ..."
      go build -o "bin/$s$EXE" "./services/$s/cmd/server" || {
        echo "编译失败：$s" >&2
        exit 1
      }
    done
    echo "==> 编译完成（5/5）"
    echo

    # ---- 2. 启动：用 healthz 判断已在跑的，跳过（不调 netstat，避免虚拟网卡导致卡顿）----
    STARTED=()
    for i in "${!SERVICES[@]}"; do
      s="${SERVICES[$i]}"
      p="${PORTS[$i]}"

      if health_ok "$p"; then
        echo "[跳过] $s (:$p) 已在运行"
      else
        nohup "./bin/$s$EXE" > "logs/$s.log" 2>&1 &
        echo "[启动中] $s (:$p)  pid=$!"
        STARTED+=("$i")
      fi
    done

    # ---- 3. 实时轮询：哪个起来就报哪个，全部就绪或超时才结束 ----
    if (( ${#STARTED[@]} > 0 )); then
      echo
      echo "==> 等待服务就绪（逐个上报，最长 60 秒）..."
      deadline=$(( $(date +%s) + 60 ))
      remaining="${#STARTED[@]}"

      while (( remaining > 0 )); do
        next=()
        for i in "${STARTED[@]}"; do
          s="${SERVICES[$i]}"
          p="${PORTS[$i]}"
          if health_ok "$p"; then
            echo "  [OK] $s (:$p) 启动完成"
            remaining=$((remaining - 1))
          else
            next+=("$i")
          fi
        done
        STARTED=("${next[@]}")
        (( remaining == 0 )) && break

        if (( $(date +%s) >= deadline )); then
          echo
          echo "==> 超时：以下服务 60 秒内未就绪，请查日志：" >&2
          for i in "${STARTED[@]}"; do
            echo "  ${SERVICES[$i]} -> tail -n 50 logs/${SERVICES[$i]}.log" >&2
          done
          exit 1
        fi
        sleep 0.5
      done
    fi

    echo
    echo "==> 全部服务启动完成（5/5）"
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
  clean)
    rm -rf bin logs
    ;;
  *)
    usage
    ;;
esac
