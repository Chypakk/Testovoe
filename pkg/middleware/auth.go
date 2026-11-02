package middleware

import (
	"net/http"
	"strings"
)

type AuthMiddleware struct {
	// Здесь будут зависимости: jwtService и т.д.
}

func NewAuthMiddleware() *AuthMiddleware {
	return &AuthMiddleware{}
}

func (m *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: Реализовать проверку JWT токена
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			http.Error(w, "Bearer token required", http.StatusUnauthorized)
			return
		}

		// TODO: Проверить валидность токена и извлечь userID
		// userID, err := m.jwtService.ValidateToken(tokenString)
		// if err != nil {
		//     http.Error(w, "Invalid token", http.StatusUnauthorized)
		//     return
		// }

		// // Добавить userID в контекст запроса
		// ctx := context.WithValue(r.Context(), "userID", userID)
		// next.ServeHTTP(w, r.WithContext(ctx))
		next.ServeHTTP(w, r)
	})
}

func (m *AuthMiddleware) RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// TODO: Реализовать проверку роли пользователя
			next.ServeHTTP(w, r)
		})
	}
}

func (m *AuthMiddleware) GetUserIDFromContext(r *http.Request) uint {
	// TODO: Извлечь userID из контекста
	return 0
}