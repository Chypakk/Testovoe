package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/render"

	"testovoe/internal/logger"
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
//
// @Summary Регистрация пользователя
// @Description Создаёт нового пользователя
// @Tags auth
// @Accept json
// @Produce json
// @Param request body models.RegisterRequest true "Данные регистрации"
// @Success 201 {object} models.User
// @Failure 400 {object} models.ErrorResponse
// @Failure 409 {object} models.ErrorResponse
// @Router /register [post]
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req models.RegisterRequest
	if err := decodeJSON(r, &req); err != nil {
		logger.Error("Failed to decode register request", err,
			"method", r.Method,
			"path", r.URL.Path,
			"ip", r.RemoteAddr,
		)
		respondError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	logger.Info("User registration attempt",
		"email", req.Email,
		"name", req.Name,
		"ip", r.RemoteAddr,
	)

	user, err := h.svc.Register(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			logger.Error("Invalid input for registration", err,
				"email", req.Email,
				"name", req.Name,
				"ip", r.RemoteAddr,
			)
			respondError(w, r, http.StatusBadRequest, "invalid input: email, name and password (min 6 chars) are required")
		case errors.Is(err, service.ErrEmailExists):
			logger.Warn("Email already registered",
				"email", req.Email,
				"name", req.Name,
				"ip", r.RemoteAddr,
			)
			respondError(w, r, http.StatusConflict, "email already registered")
		default:
			logger.Error("Registration failed", err,
				"email", req.Email,
				"name", req.Name,
				"ip", r.RemoteAddr,
			)
			respondError(w, r, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	logger.Info("User registered successfully",
		"user_id", user.ID,
		"email", user.Email,
		"name", user.Name,
		"ip", r.RemoteAddr,
	)

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, user)
}

// Login обрабатывает POST /api/v1/login.
//
// @Summary Вход пользователя
// @Description Аутентификация по email и паролю, возвращает JWT
// @Tags auth
// @Accept json
// @Produce json
// @Param request body models.LoginRequest true "Данные для входа"
// @Success 200 {object} models.LoginResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Router /login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := decodeJSON(r, &req); err != nil {
		logger.Error("Failed to decode login request", err,
			"method", r.Method,
			"path", r.URL.Path,
			"ip", r.RemoteAddr,
		)
		respondError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	logger.Info("User login attempt",
		"email", req.Email,
		"ip", r.RemoteAddr,
	)

	resp, err := h.svc.Login(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCredentials):
			logger.Warn("Invalid login credentials",
				"email", req.Email,
				"ip", r.RemoteAddr,
			)
			respondError(w, r, http.StatusUnauthorized, "invalid email or password")
		default:
			logger.Error("Login failed", err,
				"email", req.Email,
				"ip", r.RemoteAddr,
			)
			respondError(w, r, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	logger.Info("User logged in successfully",
		"user_id", resp.User.ID,
		"email", resp.User.Email,
		"ip", r.RemoteAddr,
	)

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
