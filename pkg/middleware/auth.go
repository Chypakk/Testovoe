package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/render"

	"testovoe/internal/auth"
	"testovoe/internal/models"
)

type contextKey string

const userIDKey contextKey = "userID"

type AuthMiddleware struct {
	jwt *auth.JWTService
}

func NewAuthMiddleware(jwt *auth.JWTService) *AuthMiddleware {
	return &AuthMiddleware{jwt: jwt}
}

// RequireAuth проверяет Bearer JWT и кладет userID в контекст запроса.
func (m *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			render.Status(r, http.StatusUnauthorized)
			render.JSON(w, r, models.ErrorResponse{Error: "Authorization header required"})
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			render.Status(r, http.StatusUnauthorized)
			render.JSON(w, r, models.ErrorResponse{Error: "Bearer token required"})
			return
		}

		claims, err := m.jwt.ValidateToken(tokenString)
		if err != nil || claims.UserID == 0 {
			render.Status(r, http.StatusUnauthorized)
			render.JSON(w, r, models.ErrorResponse{Error: "invalid or expired token"})
			return
		}

		ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// UserIDFromContext извлекает userID текущего пользователя из контекста.
func UserIDFromContext(ctx context.Context) uint {
	if v, ok := ctx.Value(userIDKey).(uint); ok {
		return v
	}
	return 0
}
