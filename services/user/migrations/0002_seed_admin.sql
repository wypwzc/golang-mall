-- 初始化一个管理员账号，方便本地验证管理端接口
-- 账号 admin / 密码 admin123456（生产环境请立即修改）
-- 哈希由 bcrypt(cost=10) 生成，不可逆
INSERT INTO users (username, email, password_hash, nickname, role, status, created_at, updated_at)
SELECT 'admin',
       'admin@example.com',
       '$2a$10$Nrc2gg0ruHURvyepGDg1U.wJK19wrJ/BrVLxCXDsN3P8z9CqahCCy',
       '管理员',
       'admin',
       1,
       UTC_TIMESTAMP(3),
       UTC_TIMESTAMP(3)
WHERE NOT EXISTS (SELECT 1 FROM users WHERE username = 'admin');
