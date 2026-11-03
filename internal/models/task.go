package models

import (
	"time"
)

type Task struct {
	ID          uint       `json:"id" db:"id"`
	TeamID      uint       `json:"team_id" db:"team_id"`
	Title       string     `json:"title" db:"title"`
	Description string     `json:"description" db:"description"`
	Status      string     `json:"status" db:"status"` // todo, in_progress, done
	CreatedBy   uint       `json:"created_by" db:"created_by"`
	AssigneeID  *uint      `json:"assignee_id" db:"assignee_id"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
	ClosedAt    *time.Time `json:"closed_at" db:"closed_at"`
	Version     int        `json:"version" db:"version"`
}

type CreateTaskRequest struct {
	TeamID      uint   `json:"team_id" validate:"required"`
	Title       string `json:"title" validate:"required"`
	Description string `json:"description"`
	Status      string `json:"status" validate:"required,oneof=todo in_progress done"`
	AssigneeID  *uint  `json:"assignee_id"`
}

type UpdateTaskRequest struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Status      *string `json:"status"`
	AssigneeID  *uint   `json:"assignee_id"`
}

type TaskHistory struct {
	ID        uint      `json:"id" db:"id"`
	TaskID    uint      `json:"task_id" db:"task_id"`
	ChangedBy uint      `json:"changed_by" db:"changed_by"`
	Changes   string    `json:"changes" db:"changes"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type TaskComment struct {
	ID        uint      `json:"id" db:"id"`
	TaskID    uint      `json:"task_id" db:"task_id"`
	UserID    uint      `json:"user_id" db:"user_id"`
	Content   string    `json:"content" db:"content"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type CreateCommentRequest struct {
	Content string `json:"content" validate:"required"`
}

type TeamStats struct {
	TotalTasks    int            `json:"total_tasks"`
	TasksByStatus map[string]int `json:"tasks_by_status"`
	TopPerformers []TopPerformer `json:"top_performers"`
	AvgCloseTime  float64        `json:"avg_close_time"`
	TotalComments int            `json:"total_comments"`
}

type TopPerformer struct {
	UserID    uint   `json:"user_id"`
	UserName  string `json:"user_name"`
	TaskCount int    `json:"task_count"`
}
