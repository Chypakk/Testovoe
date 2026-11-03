package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"testovoe/internal/models"
	"testovoe/internal/repository"
)

// Валидные статусы задачи.
var validTaskStatuses = map[string]bool{"todo": true, "in_progress": true, "done": true}

const topPerformersLimit = 5

// TaskService — бизнес-логика задач: создание, фильтрация, обновление с оптимистичной
// блокировкой и историей, комментарии, статистика команды.
// Все операции доступны только участникам команды; чужие задачи не видны (404).
type TaskService struct {
	tasks *repository.TaskRepo
	teams *repository.TeamRepo
	users *repository.UserRepo
}

func NewTaskService(tasks *repository.TaskRepo, teams *repository.TeamRepo, users *repository.UserRepo) *TaskService {
	return &TaskService{tasks: tasks, teams: teams, users: users}
}

// Create создает задачу в команде. AssigneeID должен быть участником этой команды.
func (s *TaskService) Create(ctx context.Context, userID uint, req models.CreateTaskRequest) (*models.Task, error) {
	title := strings.TrimSpace(req.Title)
	if req.TeamID == 0 || title == "" || !validTaskStatuses[req.Status] {
		return nil, ErrInvalidInput
	}

	if _, err := s.teams.GetMembership(ctx, req.TeamID, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrForbidden // не участник команды
		}
		return nil, err
	}

	var assignee *uint
	if req.AssigneeID != nil && *req.AssigneeID != 0 {
		role, err := s.teams.GetMembership(ctx, req.TeamID, *req.AssigneeID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, ErrInvalidInput // исполнитель не состоит в команде
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
	return created, nil
}

// List возвращает задачи по фильтрам (team_id, status, assignee_id).
func (s *TaskService) List(ctx context.Context, userID uint, f repository.TaskFilter) ([]models.Task, error) {
	if f.Status != "" && !validTaskStatuses[f.Status] {
		return nil, ErrInvalidInput
	}
	if f.TeamID != 0 {
		if _, err := s.teams.GetMembership(ctx, f.TeamID, userID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, ErrForbidden
			}
			return nil, err
		}
	}
	return s.tasks.List(ctx, f, userID)
}

// Update обновляет задачу с проверкой версии (оптимистичная блокировка).
// expectedVersion=0 означает «проверить версию по текущей записи» (мягкий режим),
// иначе при несовпадении возвращается ErrVersionConflict (HTTP 409).
func (s *TaskService) Update(ctx context.Context, taskID, userID uint, req models.UpdateTaskRequest, expectedVersion int) (*models.Task, error) {
	cur, err := s.tasks.GetVisible(ctx, taskID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTaskNotFound
		}
		return nil, err
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
		in.Title = &t
	}
	if req.Description != "" {
		d := req.Description
		in.Description = &d
	}
	if req.Status != nil {
		st := *req.Status
		if !validTaskStatuses[st] {
			return nil, ErrInvalidInput
		}
		in.Status = &st
	}
	if req.AssigneeID != nil {
		if *req.AssigneeID == 0 {
			// снять исполнителя
			in.SetAssignee = true
			in.AssigneeID = nil
		} else {
			if _, err := s.teams.GetMembership(ctx, cur.TeamID, *req.AssigneeID); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return nil, ErrInvalidInput // исполнитель не состоит в команде
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
		history = append(history, "description updated")
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
		return cur, nil // ничего не изменилось — версия не растёт
	}

	updated, err := s.tasks.UpdateWithHistory(ctx, taskID, in, history)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrVersionConflict // гонка: версия изменилась между чтением и записью
		}
		return nil, err
	}
	return updated, nil
}

// History возвращает историю изменений задачи.
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

// AddComment добавляет комментарий к задаче (только участник команды).
func (s *TaskService) AddComment(ctx context.Context, taskID, userID uint, content string) (*models.TaskComment, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, ErrInvalidInput
	}
	comment, err := s.tasks.AddComment(ctx, taskID, userID, content)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTaskNotFound // задача недоступна пользователю
		}
		return nil, err
	}
	return comment, nil
}

// Comments возвращает комментарии задачи.
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

// Stats возвращает статистику команды: распределение статусов, топ исполнителей,
// среднее время закрытия, число комментариев.
func (s *TaskService) Stats(ctx context.Context, teamID, userID uint) (*models.TeamStats, error) {
	if _, err := s.teams.GetMembership(ctx, teamID, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrForbidden
		}
		return nil, err
	}

	counters, err := s.tasks.StatsAggregate(ctx, teamID)
	if err != nil {
		return nil, err
	}
	performers, err := s.tasks.TopPerformers(ctx, teamID, topPerformersLimit)
	if err != nil {
		return nil, err
	}

	stats := &models.TeamStats{
		TotalTasks: counters.TotalTasks,
		TasksByStatus: map[string]int{
			"todo":        counters.Todo,
			"in_progress": counters.InProgress,
			"done":        counters.Done,
		},
		TopPerformers: make([]models.TopPerformer, 0, len(performers)),
		AvgCloseTime:  counters.AvgCloseSecs.Float64,
		TotalComments: counters.TotalComments,
	}
	for _, p := range performers {
		stats.TopPerformers = append(stats.TopPerformers,
			models.TopPerformer{UserID: p.UserID, UserName: p.UserName, TaskCount: p.DoneCnt})
	}
	return stats, nil
}
