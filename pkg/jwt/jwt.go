// Package jwt 封装 JWT 的签发与校验。
//
// 只支持 HS256：算法固定写死，解析时也强制校验 alg，
// 否则攻击者可以把 alg 改成 none 或 RS256 来绕过签名校验。
package jwt

import (
	"errors"
	"fmt"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"

	"go-mall/pkg/apperr"
)

// Claims 业务载荷。uid 用短字段名，减少 token 体积。
type Claims struct {
	UserID int64  `json:"uid"`
	Role   string `json:"role"`
	jwtlib.RegisteredClaims
}

// Manager 签发与解析令牌。
type Manager struct {
	secret []byte
	issuer string
	ttl    time.Duration
}

// New 创建 Manager。secret 由配置注入，不同环境必须不同。
func New(secret, issuer string, ttl time.Duration) *Manager {
	return &Manager{secret: []byte(secret), issuer: issuer, ttl: ttl}
}

// Sign 签发令牌，同时返回过期时间供前端展示。
func (m *Manager) Sign(userID int64, role string) (string, time.Time, error) {
	now := time.Now()
	expireAt := now.Add(m.ttl)

	claims := Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwtlib.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   fmt.Sprintf("%d", userID),
			IssuedAt:  jwtlib.NewNumericDate(now),
			NotBefore: jwtlib.NewNumericDate(now),
			ExpiresAt: jwtlib.NewNumericDate(expireAt),
		},
	}

	token := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, apperr.Internal.WithCause(err)
	}
	return signed, expireAt, nil
}

// Parse 校验并解析令牌。过期、签名错误、算法不符都会返回 TokenInvalid。
func (m *Manager) Parse(tokenString string) (*Claims, error) {
	claims := &Claims{}

	token, err := jwtlib.ParseWithClaims(tokenString, claims, func(t *jwtlib.Token) (any, error) {
		// 必须确认算法是我们签发时用的 HS256
		if _, ok := t.Method.(*jwtlib.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("不支持的签名算法: %v", t.Header["alg"])
		}
		return m.secret, nil
	},
		jwtlib.WithIssuer(m.issuer),
		jwtlib.WithValidMethods([]string{"HS256"}),
	)
	if err != nil {
		if errors.Is(err, jwtlib.ErrTokenExpired) {
			return nil, apperr.TokenInvalid.WithMessage("登录已过期，请重新登录")
		}
		return nil, apperr.TokenInvalid.WithCause(err)
	}
	if !token.Valid {
		return nil, apperr.TokenInvalid
	}
	return claims, nil
}
