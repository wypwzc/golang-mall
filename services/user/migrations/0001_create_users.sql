-- 用户表
-- 金额相关字段在本服务没有；密码只存 bcrypt 哈希，永不存明文
CREATE TABLE IF NOT EXISTS users (
    id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    username      VARCHAR(32)  NOT NULL COMMENT '登录名',
    email         VARCHAR(128) NOT NULL COMMENT '邮箱',
    password_hash VARCHAR(72)  NOT NULL COMMENT 'bcrypt 哈希，60 字符',
    nickname      VARCHAR(32)  NOT NULL DEFAULT '' COMMENT '昵称',
    role          VARCHAR(16)  NOT NULL DEFAULT 'user' COMMENT 'user / admin',
    status        TINYINT      NOT NULL DEFAULT 1 COMMENT '1 正常 0 禁用',
    created_at    DATETIME(3)  NOT NULL,
    updated_at    DATETIME(3)  NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_username (username),
    UNIQUE KEY uk_email (email),
    KEY idx_role_status (role, status),
    KEY idx_created_at (created_at)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT ='用户';
