package repository

import (
	"context"
	"database/sql"

	"testovoe/internal/models"
)

// UserRepo — доступ к таблице users.
type UserRepo struct {
	db  *sql.DB
	dia *Dialect
}

func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{db: db, dia: NewDialect(db)}
}

func (r *UserRepo) Create(ctx context.Context, email, passwordHash, name string) (*models.User, error) {
	query := r.dia.Ph(`INSERT INTO users (email, password_hash, name) VALUES (?, ?, ?)`) + r.dia.ReturningID()

	var id int64
	if r.dia.IsPostgres {
		if err := r.db.QueryRowContext(ctx, query, email, passwordHash, name).Scan(&id); err != nil {
			return nil, err
		}
	} else {
		res, err := r.db.ExecContext(ctx, query, email, passwordHash, name)
		if err != nil {
			return nil, err
		}
		var err2 error
		if id, err2 = res.LastInsertId(); err2 != nil {
			return nil, err2
		}
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
