package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"testovoe/internal/cache"
	"testovoe/internal/models"
	"testovoe/internal/repository"
)

// Валидные статусы задачи.
var validTaskStatuses = map[string]bool{"todo": true, "in_progress": true, "done": true}

const topPerformersLimit = 3

type TaskService struct {
	tasks     *repository.TaskRepo
	teams     *repository.TeamRepo
	users     *repository.UserRepo
	taskCache cache.TaskCache
}

func NewTaskService(tasks *repository.TaskRepo, teams *repository.TeamRepo, users *repository.UserRepo, taskCache cache.TaskCache) *TaskService {
	return &TaskService{tasks: tasks, teams: teams, users: users, taskCache: taskCache}
}

// Create создает задачу в команде и инвалидирует кеш
func (s *TaskService) Create(ctx context.Context, userID uint, req models.CreateTaskRequest) (*models.Task, error) {
	title := strings.TrimSpace(req.Title)
	if req.TeamID == 0 || title == "" || !validTaskStatuses[req.Status] {
		return nil, ErrInvalidInput
	}

	if _, err := s.teams.GetMembership(ctx, req.TeamID, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrForbidden
		}
		return nil, err
	}

	var assignee *uint
	if req.AssigneeID != nil && *req.AssigneeID != 0 {
		role, err := s.teams.GetMembership(ctx, req.TeamID, *req.AssigneeID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, ErrInvalidInput
			}
			return nil, err
		}
		_ = role
		a := *req.AssigneeID
		assignee = &a
	}

	task := &models.Task{
		TeamID:      req.TeamID,
		Title:       title,
		Description: req.Description,
		Status:      req.Status,
		CreatedBy:   userID,
		AssigneeID:  assignee,
	}

	if task.Status == "done" {
		now := nowUTC()
		task.ClosedAt = &now
	}

	created, err := s.tasks.Create(ctx, task, fmt.Sprintf("created: %q status=%s", title, req.Status))
	if err != nil {
		return nil, err
	}

	// Инвалидируем кеш для этой команды
	if err := s.taskCache.InvalidateTeam(ctx, req.TeamID); err != nil {
		log.Printf("Warning: failed to invalidate cache: %v", err)
	}

	return created, nil
}

// List возвращает задачи по фильтрам с кешированием
func (s *TaskService) List(ctx context.Context, userID uint, f repository.TaskFilter) ([]models.Task, error) {
	if f.TeamID == 0 {
		return nil, ErrInvalidInput
	}

	if f.Status != "" && !validTaskStatuses[f.Status] {
		return nil, ErrInvalidInput
	}

	if _, err := s.teams.GetMembership(ctx, f.TeamID, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrForbidden
		}
		return nil, err
	}

	// Пробуем достать из кеша
	tasks, found, err := s.taskCache.GetTasks(ctx, f)
	if err != nil {
		log.Printf("Warning: cache get error: %v", err)
	} else if found {
		return tasks, nil
	}

	// Cache miss — идем в БД
	tasks, err = s.tasks.List(ctx, f, userID)
	if err != nil {
		return nil, err
	}

	// Кладем в кеш (ошибки кеша не должны ломать ручку)
	if err := s.taskCache.SetTasks(ctx, f, tasks); err != nil {
		log.Printf("Warning: cache set error: %v", err)
	}

	return tasks, nil
}

func (s *TaskService) Count(ctx context.Context, userID uint, f repository.TaskFilter) (int, error) {
	if f.TeamID == 0 {
		return 0, ErrInvalidInput
	}

	if f.Status != "" && !validTaskStatuses[f.Status] {
		return 0, ErrInvalidInput
	}

	if _, err := s.teams.GetMembership(ctx, f.TeamID, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrForbidden
		}
		return 0, err
	}

	return s.tasks.Count(ctx, f, userID)
}

type taskEditRights struct {
	canEditAll     bool
	canChangeTitle bool
	canChangeDesc  bool
	canChangeStat  bool
	canReassign    bool
}

func resolveTaskRights(role string, task *models.Task, userID uint) taskEditRights {
	isManager := role == RoleOwner || role == RoleAdmin
	r := taskEditRights{}
	switch {
	case isManager:
		return taskEditRights{canEditAll: true, canChangeTitle: true, canChangeDesc: true, canChangeStat: true, canReassign: true}
	case task.CreatedBy == userID:
		r.canChangeTitle, r.canChangeDesc, r.canChangeStat, r.canReassign = true, true, true, true
	case task.AssigneeID != nil && *task.AssigneeID == userID:
		r.canChangeStat = true
	}
	return r
}

// Update обновляет задачу с инвалидацией кеша
func (s *TaskService) Update(ctx context.Context, taskID, userID uint, req models.UpdateTaskRequest, expectedVersion int) (*models.Task, error) {
	cur, err := s.tasks.GetVisible(ctx, taskID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTaskNotFound
		}
		return nil, err
	}

	role, err := s.teams.GetMembership(ctx, cur.TeamID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrForbidden
		}
		return nil, err
	}

	rights := resolveTaskRights(role, cur, userID)
	if !rights.canChangeTitle && !rights.canChangeDesc && !rights.canChangeStat && !rights.canReassign {
		return nil, ErrForbidden
	}

	if expectedVersion != 0 && expectedVersion != cur.Version {
		return nil, ErrVersionConflict
	}

	in := repository.UpdateInput{UserID: userID}
	if req.Title != "" {
		t := strings.TrimSpace(req.Title)
		if t == "" {
			return nil, ErrInvalidInput
		}
		if !rights.canChangeTitle {
			return nil, ErrForbidden
		}
		in.Title = &t
	}

	if req.Description != "" {
		if !rights.canChangeDesc {
			return nil, ErrForbidden
		}
		d := req.Description
		in.Description = &d
	}

	if req.Status != nil {
		st := *req.Status
		if !validTaskStatuses[st] {
			return nil, ErrInvalidInput
		}
		if !rights.canChangeStat {
			return nil, ErrForbidden
		}
		in.Status = &st
	}

	if req.AssigneeID != nil {
		if !rights.canReassign {
			return nil, ErrForbidden
		}
		if *req.AssigneeID == 0 {
			in.SetAssignee = true
			in.AssigneeID = nil
		} else {
			if _, err := s.teams.GetMembership(ctx, cur.TeamID, *req.AssigneeID); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return nil, ErrInvalidInput
				}
				return nil, err
			}
			a := *req.AssigneeID
			in.SetAssignee = true
			in.AssigneeID = &a
		}
	}

	var history []string
	if in.Title != nil && *in.Title != cur.Title {
		history = append(history, fmt.Sprintf("title: %q -> %q", cur.Title, *in.Title))
	}

	if in.Description != nil && *in.Description != cur.Description {
		history = append(history, fmt.Sprintf("description: %q -> %q", cur.Description, *in.Description))
	}
	
	if in.Status != nil && *in.Status != cur.Status {
		history = append(history, fmt.Sprintf("status: %s -> %s", cur.Status, *in.Status))
	}

	switch {
	case in.SetAssignee && in.AssigneeID == nil && cur.AssigneeID != nil:
		history = append(history, fmt.Sprintf("assignee: %d -> none", *cur.AssigneeID))
	case in.SetAssignee && in.AssigneeID != nil && (cur.AssigneeID == nil || *cur.AssigneeID != *in.AssigneeID):
		old := "none"
		if cur.AssigneeID != nil {
			old = fmt.Sprintf("%d", *cur.AssigneeID)
		}
		history = append(history, fmt.Sprintf("assignee: %s -> %d", old, *in.AssigneeID))
	}

	if len(history) == 0 {
		return cur, nil
	}

	updated, err := s.tasks.UpdateWithHistory(ctx, taskID, in, history)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrVersionConflict
		}
		return nil, err
	}

	// Инвалидируем кеш после успешного обновления
	if err := s.taskCache.InvalidateTeam(ctx, cur.TeamID); err != nil {
		log.Printf("Warning: failed to invalidate cache: %v", err)
	}

	return updated, nil
}

func (s *TaskService) History(ctx context.Context, taskID, userID uint) ([]models.TaskHistory, error) {
	items, err := s.tasks.History(ctx, taskID, userID)
	if err != nil {
		return nil, err
	}

	if len(items) == 0 {
		if _, err := s.tasks.GetVisible(ctx, taskID, userID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, ErrTaskNotFound
			}
			return nil, err
		}
	}

	return items, nil
}

func (s *TaskService) AddComment(ctx context.Context, taskID, userID uint, content string) (*models.TaskComment, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, ErrInvalidInput
	}

	comment, err := s.tasks.AddComment(ctx, taskID, userID, content)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTaskNotFound
		}
		return nil, err
	}

	return comment, nil
}

func (s *TaskService) Comments(ctx context.Context, taskID, userID uint) ([]models.TaskComment, error) {
	items, err := s.tasks.Comments(ctx, taskID, userID)
	if err != nil {
		return nil, err
	}

	if len(items) == 0 {
		if _, err := s.tasks.GetVisible(ctx, taskID, userID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, ErrTaskNotFound
			}
			return nil, err
		}
	}

	return items, nil
}

func (s *TaskService) Stats(ctx context.Context, teamID, userID uint) (*models.TeamStats, error) {
	role, err := s.teams.GetMembership(ctx, teamID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrForbidden
		}
		return nil, err
	}

	if role != RoleOwner && role != RoleAdmin {
		return nil, ErrForbidden
	}

	result, err := s.tasks.Stats(ctx, teamID, topPerformersLimit)
	if err != nil {
		return nil, err
	}

	stats := &models.TeamStats{
		TotalTasks: result.TotalTasks,
		TasksByStatus: map[string]int{
			"todo":        result.Todo,
			"in_progress": result.InProgress,
			"done":        result.Done,
		},
		TopPerformers: make([]models.TopPerformer, 0, len(result.TopPerformers)),
		AvgCloseTime:  result.AvgCloseSecs.Float64,
		TotalComments: result.TotalComments,
	}

	for _, p := range result.TopPerformers {
		stats.TopPerformers = append(stats.TopPerformers,
			models.TopPerformer{
				UserID:    p.UserID,
				UserName:  p.UserName,
				TaskCount: p.DoneCnt,
			})
	}

	return stats, nil
}
