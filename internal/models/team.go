package models

import (
	"time"
)

type Team struct {
	ID        uint      `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	CreatedBy uint      `json:"created_by" db:"created_by"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type TeamMember struct {
	ID     uint   `json:"id" db:"id"`
	TeamID uint   `json:"team_id" db:"team_id"`
	UserID uint   `json:"user_id" db:"user_id"`
	Role   string `json:"role" db:"role"` // owner, admin, member
}

type CreateTeamRequest struct {
	Name string `json:"name" validate:"required"`
}

type InviteRequest struct {
	UserID uint   `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"` // admin или member (owner не выдается через приглашение)
}

type TeamMemberWithUser struct {
	TeamID    uint   `json:"team_id" db:"team_id"`
	UserID    uint   `json:"user_id" db:"user_id"`
	Role      string `json:"role" db:"role"`
	UserName  string `json:"user_name" db:"name"`
	UserEmail string `json:"user_email" db:"email"`
}
