package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"testovoe/internal/models"
	"testovoe/internal/repository"
	"testovoe/internal/service"
	"testovoe/pkg/middleware"
)

type TaskHandler struct {
	svc *service.TaskService
}

func NewTaskHandler(svc *service.TaskService) *TaskHandler {
	return &TaskHandler{svc: svc}
}

// CreateTask обрабатывает POST /api/v1/tasks.
//
// @Summary Создание задачи
// @Description Создаёт задачу в команде. Доступно любому участнику команды
// @Tags tasks
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body models.CreateTaskRequest true "Данные задачи"
// @Success 201 {object} models.Task
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Router /tasks [post]
func (h *TaskHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
	var req models.CreateTaskRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	userID := middleware.UserIDFromContext(r.Context())
	task, err := h.svc.Create(r.Context(), userID, req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			respondError(w, r, http.StatusBadRequest, "team_id, title and valid status (todo|in_progress|done) are required; assignee must be a team member")
		case errors.Is(err, service.ErrForbidden):
			respondError(w, r, http.StatusForbidden, "you are not a member of this team")
		default:
			respondError(w, r, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, task)
}

// GetTasks обрабатывает GET /api/v1/tasks.
//
// @Summary Список задач команды
// @Description Возвращает задачи указанной команды. Требуется участие в команде
// @Tags tasks
// @Security BearerAuth
// @Produce json
// @Param team_id query int true "ID команды"
// @Param status query string false "Статус задачи" Enums(todo, in_progress, done)
// @Param assignee_id query int false "ID исполнителя"
// @Param limit query int false "Лимит записей" default(20) maximum(100)
// @Param offset query int false "Смещение" default(0) minimum(0)
// @Success 200 {object} models.PaginatedTasksResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Router /tasks [get]
func (h *TaskHandler) GetTasks(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	teamIDStr := q.Get("team_id")
	if teamIDStr == "" {
		respondError(w, r, http.StatusBadRequest, "team_id is required")
		return
	}

	teamID, err := strconv.ParseUint(teamIDStr, 10, 64)
	if err != nil || teamID == 0 {
		respondError(w, r, http.StatusBadRequest, "invalid team_id")
		return
	}

	f := repository.TaskFilter{
		TeamID: uint(teamID),
		Status: q.Get("status"),
	}

	if v := q.Get("assignee_id"); v != "" {
		id, err := strconv.ParseUint(v, 10, 64)
		if err != nil || id == 0 {
			respondError(w, r, http.StatusBadRequest, "invalid assignee_id")
			return
		}
		f.AssigneeID = uint(id)
	}

	const defaultLimit = 20
	const maxLimit = 100

	limit := defaultLimit
	if v := q.Get("limit"); v != "" {
		l, err := strconv.Atoi(v)
		if err != nil || l < 1 {
			respondError(w, r, http.StatusBadRequest, "invalid limit")
			return
		}
		limit = l
	}

	if limit > maxLimit {
		limit = maxLimit
	}

	offset := 0
	if v := q.Get("offset"); v != "" {
		o, err := strconv.Atoi(v)
		if err != nil || o < 0 {
			respondError(w, r, http.StatusBadRequest, "invalid offset")
			return
		}
		offset = o
	}

	f.Limit = limit
	f.Offset = offset

	userID := middleware.UserIDFromContext(r.Context())

	tasks, err := h.svc.List(r.Context(), userID, f)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			respondError(w, r, http.StatusBadRequest, "invalid query: team_id is required; status must be one of todo|in_progress|done")
		case errors.Is(err, service.ErrForbidden):
			respondError(w, r, http.StatusForbidden, "you are not a member of this team")
		default:
			respondError(w, r, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	total, err := h.svc.Count(r.Context(), userID, f)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			respondError(w, r, http.StatusBadRequest, "invalid query: team_id is required; status must be one of todo|in_progress|done")
		case errors.Is(err, service.ErrForbidden):
			respondError(w, r, http.StatusForbidden, "you are not a member of this team")
		default:
			respondError(w, r, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	render.JSON(w, r, models.PaginatedTasksResponse{
		Data:   tasks,
		Limit:  limit,
		Offset: offset,
		Total:  total,
	})
}

// UpdateTask обрабатывает PUT /api/v1/tasks/{id}.
//
// @Summary Обновление задачи
// @Description Обновляет задачу с проверкой прав и записью истории
// @Tags tasks
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "ID задачи"
// @Param request body object true "Данные обновления"
// @Success 200 {object} models.Task
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 409 {object} models.ErrorResponse
// @Router /tasks/{id} [put]
func (h *TaskHandler) UpdateTask(w http.ResponseWriter, r *http.Request) {
	taskID, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil || taskID == 0 {
		respondError(w, r, http.StatusBadRequest, "invalid task id")
		return
	}

	var body struct {
		models.UpdateTaskRequest
		Version int `json:"version"`
	}
	if err := decodeJSON(r, &body); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	userID := middleware.UserIDFromContext(r.Context())
	task, err := h.svc.Update(r.Context(), uint(taskID), userID, body.UpdateTaskRequest, body.Version)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTaskNotFound), errors.Is(err, sql.ErrNoRows):
			respondError(w, r, http.StatusNotFound, "task not found")
		case errors.Is(err, service.ErrVersionConflict):
			respondError(w, r, http.StatusConflict, "task was modified by someone else; reload and retry")
		case errors.Is(err, service.ErrForbidden):
			respondError(w, r, http.StatusForbidden, "you cannot edit this task: owner/admin edit any task; the creator edits their tasks; the assignee may only change status")
		case errors.Is(err, service.ErrInvalidInput):
			respondError(w, r, http.StatusBadRequest, "invalid input: status must be todo|in_progress|done; assignee must be a team member; title cannot be empty")
		default:
			respondError(w, r, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	render.JSON(w, r, task)
}

// GetTaskHistory обрабатывает GET /api/v1/tasks/{id}/history.
//
// @Summary История задачи
// @Description Возвращает историю изменений задачи. Доступно участникам команды задачи
// @Tags tasks
// @Security BearerAuth
// @Produce json
// @Param id path int true "ID задачи"
// @Success 200 {array} models.TaskHistory
// @Failure 401 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /tasks/{id}/history [get]
func (h *TaskHandler) GetTaskHistory(w http.ResponseWriter, r *http.Request) {
	taskID, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil || taskID == 0 {
		respondError(w, r, http.StatusBadRequest, "invalid task id")
		return
	}

	userID := middleware.UserIDFromContext(r.Context())
	items, err := h.svc.History(r.Context(), uint(taskID), userID)
	if err != nil {
		respondTaskAccess(w, r, err)
		return
	}

	render.JSON(w, r, items)
}

// CreateComment обрабатывает POST /api/v1/tasks/{id}/comments.
//
// @Summary Добавить комментарий
// @Description Добавляет комментарий к задаче. Доступно участникам команды задачи
// @Tags tasks
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "ID задачи"
// @Param request body models.CreateCommentRequest true "Текст комментария"
// @Success 201 {object} models.TaskComment
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /tasks/{id}/comments [post]
func (h *TaskHandler) CreateComment(w http.ResponseWriter, r *http.Request) {
	taskID, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil || taskID == 0 {
		respondError(w, r, http.StatusBadRequest, "invalid task id")
		return
	}

	var req models.CreateCommentRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	userID := middleware.UserIDFromContext(r.Context())
	comment, err := h.svc.AddComment(r.Context(), uint(taskID), userID, req.Content)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTaskNotFound):
			respondError(w, r, http.StatusNotFound, "task not found")
		case errors.Is(err, service.ErrInvalidInput):
			respondError(w, r, http.StatusBadRequest, "comment content is required")
		default:
			respondError(w, r, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, comment)
}

// GetComments обрабатывает GET /api/v1/tasks/{id}/comments.
//
// @Summary Комментарии задачи
// @Description Возвращает список комментариев задачи. Доступно участникам команды задачи
// @Tags tasks
// @Security BearerAuth
// @Produce json
// @Param id path int true "ID задачи"
// @Success 200 {array} models.TaskComment
// @Failure 401 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /tasks/{id}/comments [get]
func (h *TaskHandler) GetComments(w http.ResponseWriter, r *http.Request) {
	taskID, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil || taskID == 0 {
		respondError(w, r, http.StatusBadRequest, "invalid task id")
		return
	}

	userID := middleware.UserIDFromContext(r.Context())
	items, err := h.svc.Comments(r.Context(), uint(taskID), userID)
	if err != nil {
		respondTaskAccess(w, r, err)
		return
	}

	render.JSON(w, r, items)
}

// GetTeamStats обрабатывает GET /api/v1/teams/{team_id}/stats.
//
// @Summary SQL-отчёт по команде
// @Description Возвращает статистику команды: задачи по статусам, топ-3 исполнителей, среднее время закрытия, количество комментариев. Доступно только owner или admin
// @Tags analytics
// @Security BearerAuth
// @Produce json
// @Param team_id path int true "ID команды"
// @Success 200 {object} models.TeamStats
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Router /teams/{team_id}/stats [get]
func (h *TaskHandler) GetTeamStats(w http.ResponseWriter, r *http.Request) {
	teamID, err := strconv.ParseUint(chi.URLParam(r, "team_id"), 10, 64)
	if err != nil || teamID == 0 {
		respondError(w, r, http.StatusBadRequest, "invalid team id")
		return
	}

	userID := middleware.UserIDFromContext(r.Context())
	stats, err := h.svc.Stats(r.Context(), uint(teamID), userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrForbidden):
			respondError(w, r, http.StatusForbidden, "only team owner or admin can view analytics")
		default:
			respondError(w, r, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	render.JSON(w, r, stats)
}

func respondTaskAccess(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, service.ErrTaskNotFound), errors.Is(err, sql.ErrNoRows):
		respondError(w, r, http.StatusNotFound, "task not found")
	default:
		respondError(w, r, http.StatusInternalServerError, "internal server error")
	}
}
