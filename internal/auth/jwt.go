package auth

import (
	"kanban-management/config"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func GenerateToken(userID string) (string, error) {
	cfg := config.Get()

	now := time.Now()

	claims := jwt.MapClaims{
		"user_id": userID,
		"sub":     userID,
		"iat":     now.Unix(),
		"exp":     now.Add(time.Duration(cfg.JWTExpiresHours) * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString([]byte(cfg.JWTSecret))
}

func ValidateToken(tokenString string) (*jwt.Token, error) {
	token, err := jwt.Parse(
		tokenString,
		func(token *jwt.Token) (any, error) {
			return []byte(config.Get().JWTSecret), nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
	)

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}

	return token, nil
}
