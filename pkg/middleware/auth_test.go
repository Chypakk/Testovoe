package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"testovoe/internal/auth"
)

func TestRequireAuth_MissingToken(t *testing.T) {
	m := NewAuthMiddleware(auth.NewJWTService("secret"))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)

	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	m.RequireAuth(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if called {
		t.Error("next handler must not be called without a token")
	}
}

func TestRequireAuth_InvalidToken(t *testing.T) {
	m := NewAuthMiddleware(auth.NewJWTService("secret"))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer garbage.token.value")

	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	m.RequireAuth(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if called {
		t.Error("next handler must not be called with an invalid token")
	}
}

func TestRequireAuth_ValidToken(t *testing.T) {
	jwtSvc := auth.NewJWTService("secret")
	token, err := jwtSvc.GenerateToken(99, "user@example.com")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	m := NewAuthMiddleware(jwtSvc)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	var gotUserID uint
	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		gotUserID = UserIDFromContext(r.Context())
	})

	m.RequireAuth(handler).ServeHTTP(rec, req)

	if !called {
		t.Fatal("next handler should be called with a valid token")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if gotUserID != 99 {
		t.Errorf("UserIDFromContext = %d, want 99", gotUserID)
	}
}

func TestUserIDFromContext_NoValue(t *testing.T) {
	if id := UserIDFromContext(context.Background()); id != 0 {
		t.Errorf("UserIDFromContext(empty ctx) = %d, want 0", id)
	}
}
