package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var jwtSecret []byte

func InitJWT(secret string) {
	jwtSecret = []byte(secret)
}

type AuthRole string

const (
	Admin   AuthRole = "admin"
	Teacher AuthRole = "teacher"
	Student AuthRole = "student"
)

type Claims struct {
	UserID               string   `json:"user_id"` // 🔴 修正：从 uint 改为 string
	Role                 AuthRole `json:"role"`
	jwt.RegisteredClaims `json:"registered_claims"`
}

// GenerateToken 生成 Token
func GenerateToken(userID string, role AuthRole) (string, error) { // 🔴 修正参数类型
	claims := Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(30 * 24 * time.Hour)), // 30天有效期
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(jwtSecret)
}

func ParseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("invalid token")
}
