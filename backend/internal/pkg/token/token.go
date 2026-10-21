package token

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims JWT 载荷。TokenVersion 用于强制改密/改密后使旧 Token 全失效。
type Claims struct {
	UserID       uint   `json:"uid"`
	Role         string `json:"role"`
	TokenVersion int64  `json:"tv"`
	jwt.RegisteredClaims
}

// Generate 签发访问令牌。
func Generate(secret string, userID uint, role string, version int64, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:       userID,
		Role:         role,
		TokenVersion: version,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			Subject:   "tzwall-access",
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// Parse 校验并解析令牌。
func Parse(secret, tok string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tok, claims, func(t *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	return claims, nil
}
