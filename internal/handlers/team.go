package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"testovoe/internal/models"
	"testovoe/internal/service"
	"testovoe/pkg/middleware"
)

type TeamHandler struct {
	svc *service.TeamService
}

func NewTeamHandler(svc *service.TeamService) *TeamHandler {
	return &TeamHandler{svc: svc}
}

// CreateTeam обрабатывает POST /api/v1/teams.
func (h *TeamHandler) CreateTeam(w http.ResponseWriter, r *http.Request) {
	var req models.CreateTeamRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	userID := middleware.UserIDFromContext(r.Context())
	team, err := h.svc.Create(r.Context(), userID, req.Name)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			respondError(w, r, http.StatusBadRequest, "team name is required (max 255 chars)")
		default:
			respondError(w, r, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, team)
}

// GetTeams обрабатывает GET /api/v1/teams — только команды текущего пользователя.
func (h *TeamHandler) GetTeams(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	teams, err := h.svc.List(r.Context(), userID)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "internal server error")
		return
	}

	render.JSON(w, r, teams)
}

// InviteUser обрабатывает POST /api/v1/teams/{id}/invite.
func (h *TeamHandler) InviteUser(w http.ResponseWriter, r *http.Request) {
	teamID, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil || teamID == 0 {
		respondError(w, r, http.StatusBadRequest, "invalid team id")
		return
	}

	var req models.InviteRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	userID := middleware.UserIDFromContext(r.Context())
	member, err := h.svc.Invite(r.Context(), uint(teamID), userID, req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrForbidden):
			respondError(w, r, http.StatusForbidden, "only team owner or admin can invite users; owner role cannot be granted or changed")
		case errors.Is(err, service.ErrUserNotFound):
			respondError(w, r, http.StatusNotFound, "user not found")
		case errors.Is(err, service.ErrInvalidInput):
			respondError(w, r, http.StatusBadRequest, "role must be admin or member; user_id or email is required")
		default:
			respondError(w, r, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, member)
}

// GetMembers обрабатывает GET /api/v1/teams/{id}/members.
func (h *TeamHandler) GetMembers(w http.ResponseWriter, r *http.Request) {
	teamID, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil || teamID == 0 {
		respondError(w, r, http.StatusBadRequest, "invalid team id")
		return
	}

	userID := middleware.UserIDFromContext(r.Context())
	members, err := h.svc.Members(r.Context(), uint(teamID), userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrForbidden), errors.Is(err, sql.ErrNoRows):
			respondError(w, r, http.StatusForbidden, "access denied")
		default:
			respondError(w, r, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	render.JSON(w, r, members)
}
