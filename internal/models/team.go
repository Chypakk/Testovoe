package models

import (
	"time"
)

// Team represents a team in the system
// @Description Team information and metadata
type Team struct {
	ID        uint      `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	CreatedBy uint      `json:"created_by" db:"created_by"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// TeamMember represents a user's membership in a team
// @Description Team membership information
type TeamMember struct {
	ID     uint   `json:"id" db:"id"`
	TeamID uint   `json:"team_id" db:"team_id"`
	UserID uint   `json:"user_id" db:"user_id"`
	Role   string `json:"role" db:"role"` // owner, admin, member
}

// CreateTeamRequest represents the request for creating a new team
// @Request to create a new team
type CreateTeamRequest struct {
	Name string `json:"name" validate:"required"`
}

// InviteRequest represents the request for inviting a user to a team
// @Request to invite a user to join a team
type InviteRequest struct {
	UserID uint   `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"` // admin или member (owner не выдается через приглашение)
}

// TeamMemberWithUser represents team member with user details
// @Response containing team member and user information
type TeamMemberWithUser struct {
	TeamID    uint   `json:"team_id" db:"team_id"`
	UserID    uint   `json:"user_id" db:"user_id"`
	Role      string `json:"role" db:"role"`
	UserName  string `json:"user_name" db:"name"`
	UserEmail string `json:"user_email" db:"email"`
}
