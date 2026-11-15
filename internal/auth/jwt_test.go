package auth

import (
	"strings"
	"testing"
	"time"

	"kanban-management/config"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "0123456789abcdef0123456789abcdef"

func setup(t *testing.T) {
	t.Helper()
	t.Setenv("MONGODB_URI", "mongodb://localhost:27017")
	t.Setenv("DATABASE_NAME", "kanban_test")
	t.Setenv("JWT_SECRET", testSecret)
	t.Setenv("JWT_EXPIRES_HOURS", "1")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
}

func sign(t *testing.T, method jwt.SigningMethod, key any, claims jwt.MapClaims) string {
	t.Helper()
	s, err := jwt.NewWithClaims(method, claims).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestGenerateAndValidate(t *testing.T) {
	setup(t)
	s, err := GenerateToken("abc")
	if err != nil {
		t.Fatal(err)
	}
	token, err := ValidateToken(s)
	if err != nil {
		t.Fatal(err)
	}
	claims := token.Claims.(jwt.MapClaims)
	if claims["user_id"] != "abc" || claims["sub"] != "abc" || claims["iat"] == nil {
		t.Errorf("claims inesperadas: %v", claims)
	}
}

func TestValidateRejects(t *testing.T) {
	setup(t)
	future := time.Now().Add(time.Hour).Unix()

	cases := map[string]string{
		"expirado":       sign(t, jwt.SigningMethodHS256, []byte(testSecret), jwt.MapClaims{"user_id": "a", "exp": time.Now().Add(-time.Hour).Unix()}),
		"segredo errado": sign(t, jwt.SigningMethodHS256, []byte(strings.Repeat("x", 32)), jwt.MapClaims{"user_id": "a", "exp": future}),
		"alg none":       sign(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, jwt.MapClaims{"user_id": "a", "exp": future}),
		"alg HS512":      sign(t, jwt.SigningMethodHS512, []byte(testSecret), jwt.MapClaims{"user_id": "a", "exp": future}),
		"sem exp":        sign(t, jwt.SigningMethodHS256, []byte(testSecret), jwt.MapClaims{"user_id": "a"}),
	}

	for name, tok := range cases {
		if _, err := ValidateToken(tok); err == nil {
			t.Errorf("%s: deveria falhar", name)
		}
	}
}
