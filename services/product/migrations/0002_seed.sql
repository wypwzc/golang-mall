-- 演示数据：分类与商品
INSERT INTO categories (name, sort, created_at, updated_at)
SELECT * FROM (
    SELECT '手机数码' AS name, 1 AS sort, UTC_TIMESTAMP(3) AS c, UTC_TIMESTAMP(3) AS u
    UNION ALL SELECT '电脑办公', 2, UTC_TIMESTAMP(3), UTC_TIMESTAMP(3)
    UNION ALL SELECT '家用电器', 3, UTC_TIMESTAMP(3), UTC_TIMESTAMP(3)
) AS seed
WHERE NOT EXISTS (SELECT 1 FROM categories LIMIT 1);

INSERT INTO products (category_id, name, description, price, stock, image_url, status, created_at, updated_at)
SELECT c.id, '演示商品 A', '用于演示的商品，可删除', 99.90, 100, '', 1, UTC_TIMESTAMP(3), UTC_TIMESTAMP(3)
FROM categories c WHERE c.name = '手机数码'
  AND NOT EXISTS (SELECT 1 FROM products LIMIT 1);

INSERT INTO products (category_id, name, description, price, stock, image_url, status, created_at, updated_at)
SELECT c.id, '演示商品 B', '用于演示的商品，可删除', 19.90, 50, '', 1, UTC_TIMESTAMP(3), UTC_TIMESTAMP(3)
FROM categories c WHERE c.name = '电脑办公'
  AND (SELECT COUNT(*) FROM products) < 2;

INSERT INTO products (category_id, name, description, price, stock, image_url, status, created_at, updated_at)
SELECT c.id, '演示商品 C', '库存很少，用于测试超卖', 5.00, 3, '', 1, UTC_TIMESTAMP(3), UTC_TIMESTAMP(3)
FROM categories c WHERE c.name = '家用电器'
  AND (SELECT COUNT(*) FROM products) < 3;
