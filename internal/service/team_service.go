package service

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"testovoe/internal/models"
	"testovoe/internal/repository"
)

// Роли участников команды.
const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleMember = "member"
)

// TeamService — бизнес-логика команд: создание, список, приглашения.
type TeamService struct {
	teams *repository.TeamRepo
	users *repository.UserRepo
}

func NewTeamService(teams *repository.TeamRepo, users *repository.UserRepo) *TeamService {
	return &TeamService{teams: teams, users: users}
}

// Create создает команду; текущий пользователь становится создателем (owner).
func (s *TeamService) Create(ctx context.Context, userID uint, name string) (*models.Team, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 255 {
		return nil, ErrInvalidInput
	}
	return s.teams.CreateWithOwner(ctx, name, userID)
}

// List возвращает только те команды, где пользователь участник или создатель.
func (s *TeamService) List(ctx context.Context, userID uint) ([]models.Team, error) {
	return s.teams.ListByUser(ctx, userID)
}

// Invite добавляет пользователя в команду (по user_id или email) либо обновляет его роль.
// Право имеет только owner или admin команды. Роль owner через приглашение не выдается,
// роль создателя команды изменить нельзя.
func (s *TeamService) Invite(ctx context.Context, teamID, inviterID uint, req models.InviteRequest) (*models.TeamMemberWithUser, error) {
	callerRole, err := s.teams.GetMembership(ctx, teamID, inviterID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrForbidden // не участник команды — не видим даже её существования
	}
	if err != nil {
		return nil, err
	}
	if callerRole != RoleOwner && callerRole != RoleAdmin {
		return nil, ErrForbidden
	}

	role := strings.TrimSpace(req.Role)
	if role == "" {
		role = RoleMember
	}
	// Роль owner через приглашение не выдается.
	if role != RoleAdmin && role != RoleMember {
		return nil, ErrInvalidInput
	}

	userID := req.UserID
	if userID == 0 && strings.TrimSpace(req.Email) != "" {
		u, err := s.users.GetByEmail(ctx, strings.ToLower(strings.TrimSpace(req.Email)))
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, ErrUserNotFound
			}
			return nil, err
		}
		userID = u.ID
	}
	if userID == 0 {
		return nil, ErrInvalidInput
	}

	exists, err := s.teams.IsUserExists(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrUserNotFound
	}

	// Нельзя менять роль создателя команды (в т.ч. админу).
	targetRole, err := s.teams.GetMembership(ctx, teamID, userID)
	if err == nil && targetRole == RoleOwner {
		return nil, ErrForbidden
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	if err := s.teams.Invite(ctx, teamID, userID, role); err != nil {
		return nil, err
	}

	members, err := s.teams.Members(ctx, teamID)
	if err != nil {
		return nil, err
	}
	for _, m := range members {
		if m.UserID == userID {
			return &m, nil
		}
	}
	return nil, ErrNotMember
}

// Members возвращает состав команды (только для участников).
func (s *TeamService) Members(ctx context.Context, teamID, userID uint) ([]models.TeamMemberWithUser, error) {
	if _, err := s.teams.GetMembership(ctx, teamID, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrForbidden
		}
		return nil, err
	}
	return s.teams.Members(ctx, teamID)
}
