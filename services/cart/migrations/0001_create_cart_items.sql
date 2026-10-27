-- 购物车表：一个用户的一个商品只占一行，重复加购累加数量
CREATE TABLE IF NOT EXISTS cart_items (
    id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id    BIGINT UNSIGNED NOT NULL COMMENT '所属用户',
    product_id BIGINT UNSIGNED NOT NULL COMMENT '商品 ID',
    quantity   INT             NOT NULL COMMENT '数量',
    created_at DATETIME(3)     NOT NULL,
    updated_at DATETIME(3)     NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_user_product (user_id, product_id)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='购物车';
