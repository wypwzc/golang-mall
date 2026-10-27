-- 订单表：金额用 DECIMAL，状态只允许 pending / paid / cancelled
CREATE TABLE IF NOT EXISTS orders (
    id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    order_no     VARCHAR(32)     NOT NULL COMMENT '业务订单号',
    user_id      BIGINT UNSIGNED NOT NULL,
    total_amount DECIMAL(10, 2)  NOT NULL COMMENT '订单总额',
    status       VARCHAR(16)     NOT NULL COMMENT 'pending 待支付 / paid 已支付 / cancelled 已取消',
    paid_at      DATETIME(3)     NULL,
    cancelled_at DATETIME(3)     NULL,
    created_at   DATETIME(3)     NOT NULL,
    updated_at   DATETIME(3)     NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_order_no (order_no),
    KEY idx_user_created (user_id, created_at),
    KEY idx_status_created (status, created_at)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='订单';

-- 订单明细：商品名称和单价都是下单时的快照，
-- 商品后来改名或改价都不会影响历史订单
CREATE TABLE IF NOT EXISTS order_items (
    id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    order_id      BIGINT UNSIGNED NOT NULL,
    product_id    BIGINT UNSIGNED NOT NULL,
    product_name  VARCHAR(128)    NOT NULL COMMENT '下单时的商品名',
    product_image VARCHAR(255)    NOT NULL DEFAULT '',
    price         DECIMAL(10, 2)  NOT NULL COMMENT '下单时的单价',
    quantity      INT             NOT NULL,
    subtotal      DECIMAL(10, 2)  NOT NULL,
    PRIMARY KEY (id),
    KEY idx_order (order_id)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='订单明细';
