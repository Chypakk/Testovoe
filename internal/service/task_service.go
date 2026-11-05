package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"testovoe/internal/cache"
	"testovoe/internal/models"
	"testovoe/internal/repository"
)

// Валидные статусы задачи.
var validTaskStatuses = map[string]bool{"todo": true, "in_progress": true, "done": true}

const topPerformersLimit = 3

// TaskService — бизнес-логика задач: создание, фильтрация, обновление с оптимистичной
// блокировкой и историей, комментарии, статистика команды.
// Все операции доступны только участникам команды; чужие задачи не видны (404).
// Права на редактирование задач реализованы здесь (см. canEditTask) согласно ТЗ:
// owner/admin — любые задачи; создатель задачи — title/description/status/assignee;
// исполнитель — только статус; остальные участники — только чтение.
type TaskService struct {
	tasks *repository.TaskRepo
	teams *repository.TeamRepo
	users *repository.UserRepo
	cache cache.Cache
}

func NewTaskService(tasks *repository.TaskRepo, teams *repository.TeamRepo, users *repository.UserRepo, cache cache.Cache) *TaskService {
	return &TaskService{tasks: tasks, teams: teams, users: users, cache: cache}
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

func (s *TaskService) Count(ctx context.Context, userID uint, f repository.TaskFilter) (int, error) {
    if f.Status != "" && !validTaskStatuses[f.Status] {
        return 0, ErrInvalidInput
    }
    if f.TeamID != 0 {
        if _, err := s.teams.GetMembership(ctx, f.TeamID, userID); err != nil {
            if errors.Is(err, sql.ErrNoRows) {
                return 0, ErrForbidden
            }
            return 0, err
        }
    }
    return s.tasks.Count(ctx, f, userID)
}

// taskEditRights — результат проверки прав на редактирование задачи:
// какие поля разрешено менять вызывающему пользователю.
type taskEditRights struct {
	canEditAll     bool // owner/admin команды: любые поля, включая переназначение
	canChangeTitle bool // создатель задачи (или owner/admin)
	canChangeDesc  bool
	canChangeStat  bool
	canReassign    bool // смена/снятие исполнителя
}

// resolveTaskRights вычисляет права пользователя на задачу по ролевой модели ТЗ:
//   - owner/admin команды — редактируют любые задачи полностью;
//   - создатель задачи — название, описание, статус, исполнитель (если тот участник команды);
//   - исполнитель — только статус, переназначать задачу он не может;
//   - остальные участники — задача доступна только для чтения.
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

// Update обновляет задачу с проверкой версии (оптимистичная блокировка).
// expectedVersion=0 означает «проверить версию по текущей записи» (мягкий режим),
// иначе при несовпадении возвращается ErrVersionConflict (HTTP 409).
// Права: см. resolveTaskRights — попытка изменить запрещённое поле → ErrForbidden (403).
func (s *TaskService) Update(ctx context.Context, taskID, userID uint, req models.UpdateTaskRequest, expectedVersion int) (*models.Task, error) {
	cur, err := s.tasks.GetVisible(ctx, taskID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTaskNotFound
		}
		return nil, err
	}

	// Роль пользователя в команде задачи (GetVisible уже гарантирует членство).
	role, err := s.teams.GetMembership(ctx, cur.TeamID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrForbidden
		}
		return nil, err
	}
	rights := resolveTaskRights(role, cur, userID)
	if !rights.canChangeTitle && !rights.canChangeDesc && !rights.canChangeStat && !rights.canReassign {
		return nil, ErrForbidden // участник, не создатель и не исполнитель
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
			return nil, ErrForbidden // исполнитель не может менять название
		}
		in.Title = &t
	}
	if req.Description != "" {
		if !rights.canChangeDesc {
			return nil, ErrForbidden // исполнитель не может менять описание
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
			return nil, ErrForbidden // переназначение доступно создателю задачи и owner/admin
		}
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
// По ТЗ аналитика доступна только создателю (owner) или администратору команды.
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

    // Теперь один вызов вместо двух
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