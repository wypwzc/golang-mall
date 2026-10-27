package config_test

import (
	"strings"
	"testing"
	"time"

	"go-mall/pkg/config"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "dev")
	t.Setenv("JWT_SECRET", "unit-test-secret")

	cfg, err := config.Load("gateway")
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("网关默认端口应为 :8080，实际 %s", cfg.HTTPAddr)
	}
	if cfg.JWT.TTL != 2*time.Hour {
		t.Fatalf("JWT 默认有效期应为 2h，实际 %v", cfg.JWT.TTL)
	}
	if cfg.Upstreams["user"] == "" {
		t.Fatal("上游地址不应为空")
	}
}

func TestProdRejectsWeakSecret(t *testing.T) {
	t.Setenv("APP_ENV", "prod")
	t.Setenv("JWT_SECRET", "dev-only-too-short")
	t.Setenv("INTERNAL_TOKEN", "token")
	t.Setenv("MYSQL_PASSWORD", "pwd")

	if _, err := config.Load("user"); err == nil {
		t.Fatal("生产环境使用开发密钥时必须拒绝启动")
	}
}

func TestProdRejectsMissingInternalToken(t *testing.T) {
	t.Setenv("APP_ENV", "prod")
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("INTERNAL_TOKEN", "")
	t.Setenv("MYSQL_PASSWORD", "pwd")

	if _, err := config.Load("user"); err == nil {
		t.Fatal("生产环境缺少 INTERNAL_TOKEN 时必须拒绝启动")
	}
}

func TestPerServiceDBPassword(t *testing.T) {
	t.Setenv("JWT_SECRET", "unit-test-secret")
	t.Setenv("MYSQL_PASSWORD", "")
	t.Setenv("USER_DB_PASSWORD", "user-pwd")
	t.Setenv("ORDER_DB_PASSWORD", "order-pwd")

	cfg, err := config.Load("user")
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}
	if cfg.MySQL.Password != "user-pwd" {
		t.Fatalf("user 服务应使用 USER_DB_PASSWORD，实际 %q", cfg.MySQL.Password)
	}
	if cfg.MySQL.User != "mall_user" || cfg.MySQL.Database != "mall_user" {
		t.Fatalf("账号与库名默认值不对: %s/%s", cfg.MySQL.User, cfg.MySQL.Database)
	}

	cfg, err = config.Load("order")
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}
	if cfg.MySQL.Password != "order-pwd" {
		t.Fatalf("order 服务应使用 ORDER_DB_PASSWORD，实际 %q", cfg.MySQL.Password)
	}
}

func TestRedisCanBeDisabled(t *testing.T) {
	t.Setenv("JWT_SECRET", "unit-test-secret")
	t.Setenv("REDIS_ENABLED", "false")

	cfg, err := config.Load("user")
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}
	if cfg.Redis.Enabled {
		t.Fatal("REDIS_ENABLED=false 时应关闭 Redis")
	}

	t.Setenv("REDIS_ENABLED", "true")
	cfg, err = config.Load("user")
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}
	if !cfg.Redis.Enabled {
		t.Fatal("REDIS_ENABLED=true 时应启用 Redis")
	}
}

func TestDSNContainsUTC(t *testing.T) {
	m := config.MySQL{User: "u", Password: "p", Host: "h", Port: "3306", Database: "d"}
	dsn := m.DSN()
	for _, want := range []string{"loc=UTC", "parseTime=True", "multiStatements=true"} {
		if !strings.Contains(dsn, want) {
			t.Fatalf("DSN 缺少 %s: %s", want, dsn)
		}
	}
}
