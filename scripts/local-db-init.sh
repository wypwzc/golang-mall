#!/usr/bin/env bash
# 在本机已安装的 MySQL 上初始化 go-mall 所需的库与账号（不使用 Docker）。
#
# 前置条件：本机 MySQL 服务已启动
#   Windows（管理员 PowerShell）：net start MySQL80
#
# 用法：
#   ./scripts/local-db-init.sh
#   或先导出 root 密码再执行：MYSQL_LOCAL_ROOT_PASSWORD=xxx ./scripts/local-db-init.sh
set -euo pipefail

cd "$(dirname "$0")/.."

if [[ ! -f .env ]]; then
  echo "错误：找不到 .env，请先执行 cp .env.example .env" >&2
  exit 1
fi

# 载入 .env 中的开发密码
set -a
# shellcheck disable=SC1091
source .env
set +a

MYSQL_HOST="${MYSQL_HOST:-127.0.0.1}"
MYSQL_PORT="${MYSQL_PORT:-3306}"

# 1. 检查 MySQL 是否在监听
if ! (echo >/dev/tcp/"$MYSQL_HOST"/"$MYSQL_PORT") 2>/dev/null; then
  cat >&2 <<EOF
错误：连接不上 $MYSQL_HOST:$MYSQL_PORT

请先启动本机 MySQL 服务（需要管理员权限）：
  PowerShell(管理员):  net start MySQL80
  或 services.msc 里启动 MySQL80
EOF
  exit 1
fi

# 2. 取 root 密码（不写进脚本，也不回显）
ROOT_PWD="${MYSQL_LOCAL_ROOT_PASSWORD:-}"
if [[ -z "$ROOT_PWD" ]]; then
  read -r -s -p "请输入本机 MySQL root 密码: " ROOT_PWD
  echo
fi

# 3. 先验证 root 能登录，避免后面报错难以定位
if ! MYSQL_PWD="$ROOT_PWD" mysql -h"$MYSQL_HOST" -P"$MYSQL_PORT" -uroot -e "SELECT 1" >/dev/null 2>&1; then
  echo "错误：root 登录失败，请检查密码是否正确" >&2
  exit 1
fi

# 4. 创建库与账号（可重复执行）
echo "==> 创建数据库与账号"
MYSQL_PWD="$ROOT_PWD" mysql -h"$MYSQL_HOST" -P"$MYSQL_PORT" -uroot <<SQL
CREATE DATABASE IF NOT EXISTS \`mall_user\`    DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS \`mall_product\` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS \`mall_cart\`    DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS \`mall_order\`   DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS \`mall_payment\` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

CREATE USER IF NOT EXISTS 'mall_user'@'%'    IDENTIFIED BY '${USER_DB_PASSWORD}';
CREATE USER IF NOT EXISTS 'mall_product'@'%' IDENTIFIED BY '${PRODUCT_DB_PASSWORD}';
CREATE USER IF NOT EXISTS 'mall_cart'@'%'    IDENTIFIED BY '${CART_DB_PASSWORD}';
CREATE USER IF NOT EXISTS 'mall_order'@'%'   IDENTIFIED BY '${ORDER_DB_PASSWORD}';
CREATE USER IF NOT EXISTS 'mall_payment'@'%' IDENTIFIED BY '${PAYMENT_DB_PASSWORD}';

ALTER USER 'mall_user'@'%'    IDENTIFIED BY '${USER_DB_PASSWORD}';
ALTER USER 'mall_product'@'%' IDENTIFIED BY '${PRODUCT_DB_PASSWORD}';
ALTER USER 'mall_cart'@'%'    IDENTIFIED BY '${CART_DB_PASSWORD}';
ALTER USER 'mall_order'@'%'   IDENTIFIED BY '${ORDER_DB_PASSWORD}';
ALTER USER 'mall_payment'@'%' IDENTIFIED BY '${PAYMENT_DB_PASSWORD}';

GRANT ALL PRIVILEGES ON \`mall_user\`.*    TO 'mall_user'@'%';
GRANT ALL PRIVILEGES ON \`mall_product\`.* TO 'mall_product'@'%';
GRANT ALL PRIVILEGES ON \`mall_cart\`.*    TO 'mall_cart'@'%';
GRANT ALL PRIVILEGES ON \`mall_order\`.*   TO 'mall_order'@'%';
GRANT ALL PRIVILEGES ON \`mall_payment\`.* TO 'mall_payment'@'%';

FLUSH PRIVILEGES;
SQL

# 5. 用业务账号验证一遍，确认权限正确
echo "==> 验证业务账号"
for pair in "mall_user:${USER_DB_PASSWORD}" "mall_product:${PRODUCT_DB_PASSWORD}" \
            "mall_cart:${CART_DB_PASSWORD}" "mall_order:${ORDER_DB_PASSWORD}" \
            "mall_payment:${PAYMENT_DB_PASSWORD}"; do
  user="${pair%%:*}"
  pwd="${pair#*:}"
  if MYSQL_PWD="$pwd" mysql -h"$MYSQL_HOST" -P"$MYSQL_PORT" -u"$user" -e "SELECT 1" >/dev/null 2>&1; then
    printf '  %-14s OK\n' "$user"
  else
    printf '  %-14s 失败\n' "$user" >&2
    exit 1
  fi
done

echo "==> 完成。现在可以启动服务：./scripts/tasks.sh run user"
