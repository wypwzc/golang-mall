#!/bin/bash
# MySQL 首次初始化脚本（仅在数据目录为空时执行一次）
# 职责：创建 5 个业务库 + 5 个只拥有自己库权限的账号
set -euo pipefail

echo "==> 初始化 go-mall 数据库与账号"

mysql -uroot -p"${MYSQL_ROOT_PASSWORD}" <<-SQL
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

	GRANT ALL PRIVILEGES ON \`mall_user\`.*    TO 'mall_user'@'%';
	GRANT ALL PRIVILEGES ON \`mall_product\`.* TO 'mall_product'@'%';
	GRANT ALL PRIVILEGES ON \`mall_cart\`.*    TO 'mall_cart'@'%';
	GRANT ALL PRIVILEGES ON \`mall_order\`.*   TO 'mall_order'@'%';
	GRANT ALL PRIVILEGES ON \`mall_payment\`.* TO 'mall_payment'@'%';

	FLUSH PRIVILEGES;
SQL

echo "==> 初始化完成"
