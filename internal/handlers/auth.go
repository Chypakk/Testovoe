package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/render"

	"testovoe/internal/models"
	"testovoe/internal/service"
)

type AuthHandler struct {
	svc *service.AuthService
}

func NewAuthHandler(svc *service.AuthService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

// Register обрабатывает POST /api/v1/register.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req models.RegisterRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := h.svc.Register(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			respondError(w, r, http.StatusBadRequest, "invalid input: email, name and password (min 6 chars) are required")
		case errors.Is(err, service.ErrEmailExists):
			respondError(w, r, http.StatusConflict, "email already registered")
		default:
			respondError(w, r, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, user)
}

// Login обрабатывает POST /api/v1/login, выдает JWT.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.svc.Login(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCredentials):
			respondError(w, r, http.StatusUnauthorized, "invalid email or password")
		default:
			respondError(w, r, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	render.JSON(w, r, resp)
}

func decodeJSON(r *http.Request, dst interface{}) error {
	body := http.MaxBytesReader(nil, r.Body, 1<<20) // 1 МБ
	defer body.Close()
	dec := json.NewDecoder(body)
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("empty body")
		}
		return err
	}
	return nil
}

func respondError(w http.ResponseWriter, r *http.Request, status int, msg string) {
	render.Status(r, status)
	render.JSON(w, r, models.ErrorResponse{Error: msg})
}
