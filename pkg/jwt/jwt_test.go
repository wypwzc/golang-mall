package jwt_test

import (
	"testing"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"

	"go-mall/pkg/jwt"
)

func TestSignAndParse(t *testing.T) {
	m := jwt.New("test-secret-at-least-32-bytes-long!!", "go-mall", time.Hour)

	token, expireAt, err := m.Sign(42, "admin")
	if err != nil {
		t.Fatalf("签发失败: %v", err)
	}
	if token == "" || expireAt.Before(time.Now()) {
		t.Fatal("令牌或过期时间不合法")
	}

	claims, err := m.Parse(token)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if claims.UserID != 42 || claims.Role != "admin" {
		t.Fatalf("载荷不正确: %+v", claims)
	}
}

func TestParseRejectsWrongSecret(t *testing.T) {
	signer := jwt.New("secret-one-at-least-32-bytes-long!!", "go-mall", time.Hour)
	verifier := jwt.New("secret-two-at-least-32-bytes-long!!", "go-mall", time.Hour)

	token, _, _ := signer.Sign(1, "user")
	if _, err := verifier.Parse(token); err == nil {
		t.Fatal("换密钥后必须校验失败")
	}
}

func TestParseRejectsExpired(t *testing.T) {
	m := jwt.New("test-secret-at-least-32-bytes-long!!", "go-mall", -time.Minute)

	token, _, _ := m.Sign(1, "user")
	if _, err := m.Parse(token); err == nil {
		t.Fatal("过期令牌必须被拒绝")
	}
}

func TestParseRejectsWrongIssuer(t *testing.T) {
	signer := jwt.New("test-secret-at-least-32-bytes-long!!", "other-issuer", time.Hour)
	verifier := jwt.New("test-secret-at-least-32-bytes-long!!", "go-mall", time.Hour)

	token, _, _ := signer.Sign(1, "user")
	if _, err := verifier.Parse(token); err == nil {
		t.Fatal("签发方不一致必须被拒绝")
	}
}

// alg=none 是 JWT 最经典的攻击方式：把算法改成 none 并去掉签名。
func TestParseRejectsNoneAlgorithm(t *testing.T) {
	m := jwt.New("test-secret-at-least-32-bytes-long!!", "go-mall", time.Hour)

	token := jwtlib.NewWithClaims(jwtlib.SigningMethodNone, jwtlib.MapClaims{
		"uid":  1,
		"role": "admin",
		"iss":  "go-mall",
		"exp":  time.Now().Add(time.Hour).Unix(),
	})
	signed, err := token.SignedString(jwtlib.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("构造测试令牌失败: %v", err)
	}

	if _, err := m.Parse(signed); err == nil {
		t.Fatal("alg=none 必须被拒绝")
	}
}
