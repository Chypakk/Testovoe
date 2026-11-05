package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-secret-key"

func TestGenerateAndValidateToken(t *testing.T) {
	svc := NewJWTService(testSecret)

	token, err := svc.GenerateToken(42, "user@example.com")
	if err != nil {
		t.Fatalf("GenerateToken: unexpected error: %v", err)
	}
	if token == "" {
		t.Fatal("GenerateToken: returned empty token")
	}

	claims, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken: unexpected error: %v", err)
	}
	if claims.UserID != 42 {
		t.Errorf("claims.UserID = %d, want 42", claims.UserID)
	}
	if claims.Email != "user@example.com" {
		t.Errorf("claims.Email = %q, want %q", claims.Email, "user@example.com")
	}
}

func TestValidateToken_WrongSecret(t *testing.T) {
	signer := NewJWTService(testSecret)
	token, err := signer.GenerateToken(1, "a@b.com")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	validator := NewJWTService("another-secret")
	if _, err := validator.ValidateToken(token); err == nil {
		t.Fatal("expected error when validating token with wrong secret, got nil")
	}
}

func TestValidateToken_MalformedToken(t *testing.T) {
	svc := NewJWTService(testSecret)

	for _, bad := range []string{"", "not-a-token", "a.b.c"} {
		if _, err := svc.ValidateToken(bad); err == nil {
			t.Errorf("ValidateToken(%q): expected error, got nil", bad)
		}
	}
}

func TestGeneratedToken_HasFutureExpirationAndJWTFormat(t *testing.T) {
	svc := NewJWTService(testSecret)

	token, err := svc.GenerateToken(7, "user@example.com")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if strings.Count(token, ".") != 2 {
		t.Errorf("token should have JWT format (3 dot-separated parts), got %q", token)
	}

	claims, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if claims.ExpiresAt == nil || !claims.ExpiresAt.Time.After(time.Now()) {
		t.Error("expected token to have expiration in the future")
	}
}

func TestValidateToken_ExpiredToken(t *testing.T) {
	svc := NewJWTService(testSecret)

	// Подписываем токен, который уже истёк, и убеждаемся, что он отклоняется.
	claims := &Claims{
		UserID: 7,
		Email:  "expired@example.com",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("signing expired token: %v", err)
	}

	if _, err := svc.ValidateToken(token); err == nil {
		t.Fatal("expected expired token to be rejected, got nil error")
	}
}
