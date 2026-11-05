package cache

import (
	"context"
	"testovoe/internal/models"
	"testovoe/internal/repository"
)

// TaskCache — интерфейс для кеширования списка задач
type TaskCache interface {
	GetTasks(ctx context.Context, filter repository.TaskFilter) ([]models.Task, bool, error)
	SetTasks(ctx context.Context, filter repository.TaskFilter, tasks []models.Task) error
	InvalidateTeam(ctx context.Context, teamID uint) error
}