package repository

import (
	"context"
	"database/sql"

	"testovoe/internal/models"
)

// UserRepo — доступ к таблице users.
type UserRepo struct {
	db *sql.DB
}

func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) Create(ctx context.Context, email, passwordHash, name string) (*models.User, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO users (email, password_hash, name) VALUES (?, ?, ?)`,
		email, passwordHash, name)
	if err != nil {
		return nil, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	return r.GetByID(ctx, uint(id))
}

func (r *UserRepo) GetByID(ctx context.Context, id uint) (*models.User, error) {
	u := &models.User{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash, name, created_at FROM users WHERE id = ?`, id).
		Scan(&u.ID, &u.Email, &u.Password, &u.Name, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, sql.ErrNoRows
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	u := &models.User{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash, name, created_at FROM users WHERE email = ?`, email).
		Scan(&u.ID, &u.Email, &u.Password, &u.Name, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, sql.ErrNoRows
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}
