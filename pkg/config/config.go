// Package config 负责加载配置。
//
// 优先级：环境变量 > .env 文件 > 代码内默认值。
// 容器里由 docker-compose 注入环境变量，本地开发读项目根目录的 .env。
package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// MySQL 数据库连接配置。
type MySQL struct {
	Host            string
	Port            string
	User            string
	Password        string
	Database        string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// Redis 缓存配置。
// Enabled=false 时服务不连接 Redis，用于本地无 Redis 环境的开发调试。
type Redis struct {
	Enabled  bool
	Addr     string
	Password string
	DB       int
}

// JWT 令牌配置。Secret 由 user 服务签发、网关验签，两边必须一致。
type JWT struct {
	Secret string
	Issuer string
	TTL    time.Duration
}

// Config 所有服务的配置集合。每个服务只使用其中一部分。
type Config struct {
	Env           string // dev | prod
	ServiceName   string
	HTTPAddr      string
	LogLevel      string
	MySQL         MySQL
	Redis         Redis
	JWT           JWT
	InternalToken string   // 服务间内部接口的共享令牌
	CORSOrigins   []string // 允许跨域的来源
	Upstreams     map[string]string

	// 登录失败限流：同一账号在 LoginFailWindow 内失败超过 LoginMaxFails 次即锁定
	LoginMaxFails   int
	LoginFailWindow time.Duration
}

// 各服务的默认监听端口，可通过 HTTP_ADDR 覆盖。
var defaultPorts = map[string]string{
	"gateway": "8080",
	"user":    "8081",
	"product": "8082",
	"cart":    "8083",
	"order":   "8084",
	"payment": "8085",
}

// 每个业务库使用独立账号，因此密码变量名也不同。
// 这样即使某个服务的配置泄漏，也只影响它自己那个库。
var dbPasswordEnv = map[string]string{
	"user":    "USER_DB_PASSWORD",
	"product": "PRODUCT_DB_PASSWORD",
	"cart":    "CART_DB_PASSWORD",
	"order":   "ORDER_DB_PASSWORD",
	"payment": "PAYMENT_DB_PASSWORD",
}

// Load 加载指定服务的配置。service 取值：gateway/user/product/cart/order/payment。
func Load(service string) (*Config, error) {
	// .env 只在本地开发时存在；容器里由 compose 注入环境变量，读不到就算了。
	_ = loadDotEnv(".env")

	// 优先读通用的 MYSQL_PASSWORD，没有则读该服务专属的密码变量
	dbPassword := getenv("MYSQL_PASSWORD", "")
	if dbPassword == "" {
		dbPassword = getenv(dbPasswordEnv[service], "")
	}
	cfg := &Config{
		Env:         getenv("APP_ENV", "dev"),
		ServiceName: service,
		HTTPAddr:    getenv("HTTP_ADDR", ":"+defaultPorts[service]),
		LogLevel:    getenv("LOG_LEVEL", "info"),
		MySQL: MySQL{
			Host:            getenv("MYSQL_HOST", "127.0.0.1"),
			Port:            getenv("MYSQL_PORT", "3306"),
			User:            getenv("MYSQL_USER", "mall_"+service),
			Password:        dbPassword,
			Database:        getenv("MYSQL_DATABASE", "mall_"+service),
			MaxOpenConns:    getenvInt("MYSQL_MAX_OPEN_CONNS", 20),
			MaxIdleConns:    getenvInt("MYSQL_MAX_IDLE_CONNS", 10),
			ConnMaxLifetime: getenvDuration("MYSQL_CONN_MAX_LIFETIME", time.Hour),
		},
		Redis: Redis{
			Enabled:  getenvBool("REDIS_ENABLED", true),
			Addr:     getenv("REDIS_ADDR", "127.0.0.1:6379"),
			Password: getenv("REDIS_PASSWORD", ""),
			DB:       getenvInt("REDIS_DB", 0),
		},
		JWT: JWT{
			Secret: getenv("JWT_SECRET", ""),
			Issuer: getenv("JWT_ISSUER", "go-mall"),
			TTL:    getenvDuration("JWT_TTL", 2*time.Hour),
		},
		InternalToken: getenv("INTERNAL_TOKEN", ""),
		CORSOrigins:   splitCSV(getenv("CORS_ORIGINS", "http://localhost:5173,http://127.0.0.1:5173")),

		LoginMaxFails:   getenvInt("LOGIN_MAX_FAILS", 5),
		LoginFailWindow: getenvDuration("LOGIN_FAIL_WINDOW", 5*time.Minute),
		Upstreams: map[string]string{
			"user":    getenv("UPSTREAM_USER", "http://127.0.0.1:8081"),
			"product": getenv("UPSTREAM_PRODUCT", "http://127.0.0.1:8082"),
			"cart":    getenv("UPSTREAM_CART", "http://127.0.0.1:8083"),
			"order":   getenv("UPSTREAM_ORDER", "http://127.0.0.1:8084"),
			"payment": getenv("UPSTREAM_PAYMENT", "http://127.0.0.1:8085"),
		},
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// IsProd 是否为生产环境。
func (c *Config) IsProd() bool { return c.Env == "prod" }

// validate 生产环境的关键配置校验，防止带着默认密钥上线。
func (c *Config) validate() error {
	if c.IsProd() {
		if len(c.JWT.Secret) < 32 || strings.Contains(c.JWT.Secret, "dev-only") {
			return fmt.Errorf("生产环境 JWT_SECRET 必须至少 32 位且不能使用开发默认值")
		}
		if c.InternalToken == "" {
			return fmt.Errorf("生产环境必须设置 INTERNAL_TOKEN")
		}
		if c.ServiceName != "gateway" && c.MySQL.Password == "" {
			return fmt.Errorf("生产环境必须设置 MYSQL_PASSWORD")
		}
	}
	if c.JWT.Secret == "" {
		return fmt.Errorf("JWT_SECRET 未配置，请参考 .env.example")
	}
	return nil
}

// DSN 生成 GORM 使用的 MySQL 连接串。
// multiStatements=true 是为了让迁移脚本可以一次执行多条语句。
// loc=UTC 保证时间统一按 UTC 存取，避免跨时区偏移。
func (m MySQL) DSN() string {
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=UTC&multiStatements=true&timeout=5s&readTimeout=10s&writeTimeout=10s",
		m.User, m.Password, m.Host, m.Port, m.Database,
	)
}

func getenv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getenvBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func getenvDuration(key string, def time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// loadDotEnv 读取 key=value 格式的 .env，不覆盖已存在的环境变量。
func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
	return sc.Err()
}
