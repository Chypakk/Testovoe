package handlers

import (
	"net/http"

	"github.com/go-chi/render"
)

type AuthHandler struct {
	// Здесь будут зависимости: userService, jwtService и т.д.
}

func NewAuthHandler() *AuthHandler {
	return &AuthHandler{}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	// TODO: Реализовать регистрацию пользователя
	render.JSON(w, r, map[string]string{"message": "Register endpoint - TODO"})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	// TODO: Реализовать аутентификацию пользователя
	render.JSON(w, r, map[string]string{"message": "Login endpoint - TODO"})
}