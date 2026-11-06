package service

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"

	"testovoe/internal/auth"
	"testovoe/internal/logger"
	"testovoe/internal/models"
	"testovoe/internal/repository"

	"golang.org/x/crypto/bcrypt"
)

// Ошибки уровня сервиса.
var (
	ErrInvalidInput       = errors.New("invalid input")
	ErrEmailExists        = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUserNotFound       = errors.New("user not found")
	ErrTeamNotFound       = errors.New("team not found")
	ErrForbidden          = errors.New("forbidden")
	ErrNotMember          = errors.New("user is not a member of this team")
	ErrTaskNotFound       = errors.New("task not found")
	ErrVersionConflict    = errors.New("task was modified concurrently")
)

const (
	dupEntryErrNum    = 1062    // MySQL duplicate entry
	pgUniqueViolation = "23505" // PostgreSQL unique_violation (SQLSTATE)
)

// isDuplicateKey определяет нарушение уникальности для обоих поддерживаемых драйверов.
func isDuplicateKey(err error) bool {
	var me *mysql.MySQLError
	if errors.As(err, &me) && me.Number == dupEntryErrNum { // MySQL 1062
		return true
	}
	var pe *pgconn.PgError
	if errors.As(err, &pe) && pe.Code == pgUniqueViolation { // PostgreSQL 23505
		return true
	}
	return false
}

// AuthService — регистрация, аутентификация, выдача JWT.
type AuthService struct {
	users *repository.UserRepo
	jwt   *auth.JWTService
}

func NewAuthService(users *repository.UserRepo, jwt *auth.JWTService) *AuthService {
	return &AuthService{users: users, jwt: jwt}
}

func (s *AuthService) Register(ctx context.Context, req models.RegisterRequest) (*models.User, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	name := strings.TrimSpace(req.Name)

	logger.Info("Processing user registration",
		"email", email,
		"name", name,
	)

	if !validEmail(email) {
		logger.Error("Invalid email format", nil,
			"email", email,
			"name", name,
		)
		return nil, ErrInvalidInput
	}
	if len(req.Password) < 6 {
		logger.Error("Password too short", nil,
			"email", email,
			"name", name,
		)
		return nil, ErrInvalidInput
	}
	if name == "" {
		logger.Error("Empty name", nil,
			"email", email,
			"name", name,
		)
		return nil, ErrInvalidInput
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		logger.Error("Failed to hash password", err,
			"email", email,
			"name", name,
		)
		return nil, err
	}

	user, err := s.users.Create(ctx, email, string(hash), name)
	if err != nil {
		if isDuplicateKey(err) {
			logger.Warn("Email already exists", err,
				"email", email,
				"name", name,
			)
			return nil, ErrEmailExists
		}
		logger.Error("Failed to create user", err,
			"email", email,
			"name", name,
		)
		return nil, err
	}

	logger.Info("User created successfully",
		"user_id", user.ID,
		"email", user.Email,
		"name", user.Name,
	)
	return user, nil
}

func (s *AuthService) Login(ctx context.Context, req models.LoginRequest) (*models.LoginResponse, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))

	logger.Info("Processing user login",
		"email", email,
	)

	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			logger.Warn("User not found", err,
				"email", email,
			)
			return nil, ErrInvalidCredentials
		}
		logger.Error("Failed to get user by email", err,
			"email", email,
		)
		return nil, err
	}

	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)) != nil {
		logger.Warn("Invalid password", nil,
			"email", email,
			"user_id", user.ID,
		)
		return nil, ErrInvalidCredentials
	}

	token, err := s.jwt.GenerateToken(user.ID, user.Email)
	if err != nil {
		logger.Error("Failed to generate token", err,
			"email", email,
			"user_id", user.ID,
		)
		return nil, err
	}

	logger.Info("User logged in successfully",
		"email", email,
		"user_id", user.ID,
	)

	return &models.LoginResponse{Token: token, User: *user}, nil
}

func validEmail(email string) bool {
	if email == "" || len(email) > 255 {
		return false
	}
	i := strings.Index(email, "@")
	if i <= 0 || i == len(email)-1 || strings.Contains(email[i+1:], "@") {
		return false
	}
	domain := email[i+1:]
	j := strings.Index(domain, ".")
	if j <= 0 || j == len(domain)-1 {
		return false
	}
	return !strings.ContainsAny(email, " \t")
}
