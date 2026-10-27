-- 商品库：分类、商品、库存扣减账本

CREATE TABLE IF NOT EXISTS categories (
    id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    name       VARCHAR(64)     NOT NULL COMMENT '分类名',
    sort       INT             NOT NULL DEFAULT 0 COMMENT '排序，越小越前',
    created_at DATETIME(3)     NOT NULL,
    updated_at DATETIME(3)     NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_name (name)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='商品分类';

CREATE TABLE IF NOT EXISTS products (
    id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    category_id BIGINT UNSIGNED NOT NULL COMMENT '所属分类',
    name        VARCHAR(128)    NOT NULL,
    description TEXT            NULL,
    price       DECIMAL(10, 2)  NOT NULL COMMENT '单价，精确到分',
    stock       INT             NOT NULL DEFAULT 0 COMMENT '库存，扣减由条件更新保证不超卖',
    image_url   VARCHAR(255)    NOT NULL DEFAULT '',
    status      TINYINT         NOT NULL DEFAULT 1 COMMENT '1 上架 0 下架',
    created_at  DATETIME(3)     NOT NULL,
    updated_at  DATETIME(3)     NOT NULL,
    PRIMARY KEY (id),
    KEY idx_category_status (category_id, status),
    KEY idx_status_created (status, created_at),
    KEY idx_name (name)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='商品';

-- 库存扣减账本：order_no + product_id 唯一，是"扣减/恢复"幂等的基础
CREATE TABLE IF NOT EXISTS stock_deductions (
    id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    order_no   VARCHAR(32)     NOT NULL COMMENT '订单号，作为幂等键',
    product_id BIGINT UNSIGNED NOT NULL,
    quantity   INT             NOT NULL,
    status     VARCHAR(16)     NOT NULL COMMENT 'deducted / restored',
    created_at DATETIME(3)     NOT NULL,
    updated_at DATETIME(3)     NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_order_product (order_no, product_id),
    KEY idx_order (order_no)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='库存扣减流水';
