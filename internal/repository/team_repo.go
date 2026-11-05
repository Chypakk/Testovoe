package repository

import (
	"context"
	"database/sql"

	"testovoe/internal/configs"
	"testovoe/internal/models"
)

// TeamRepo — доступ к таблицам teams и team_members.
type TeamRepo struct {
	db  *sql.DB
	dia *Dialect
}

func NewTeamRepo(db *sql.DB, cfg *configs.Config) *TeamRepo {
	return &TeamRepo{db: db, dia: NewDialectFromDriver(cfg.Database.Driver)}
}

// CreateWithOwner атомарно создает команду и добавляет создателя как участника с ролью owner.
func (r *TeamRepo) CreateWithOwner(ctx context.Context, name string, ownerID uint) (*models.Team, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var teamID int64
	if r.dia.IsPostgres {
		err = tx.QueryRowContext(ctx,
			r.dia.Ph(`INSERT INTO teams (name, created_by) VALUES (?, ?) RETURNING id`),
			name, ownerID).Scan(&teamID)
	} else {
		var res sql.Result
		res, err = tx.ExecContext(ctx,
			r.dia.Ph(`INSERT INTO teams (name, created_by) VALUES (?, ?)`), name, ownerID)
		if err == nil {
			teamID, err = res.LastInsertId()
		}
	}
	if err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx,
		r.dia.Ph(`INSERT INTO team_members (team_id, user_id, role) VALUES (?, ?, 'owner')`),
		teamID, ownerID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return r.GetByID(ctx, uint(teamID))
}

func (r *TeamRepo) GetByID(ctx context.Context, id uint) (*models.Team, error) {
	t := &models.Team{}
	err := r.db.QueryRowContext(ctx,
		r.dia.Ph(`SELECT id, name, created_by, created_at FROM teams WHERE id = ?`), id).
		Scan(&t.ID, &t.Name, &t.CreatedBy, &t.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, sql.ErrNoRows
	}
	if err != nil {
		return nil, err
	}
	return t, nil
}

// ListByUser возвращает команды, где пользователь является участником (или создателем).
func (r *TeamRepo) ListByUser(ctx context.Context, userID uint) ([]models.Team, error) {
	rows, err := r.db.QueryContext(ctx,
		r.dia.Ph(`SELECT DISTINCT t.id, t.name, t.created_by, t.created_at
         FROM teams t
         JOIN team_members tm ON tm.team_id = t.id
         WHERE tm.user_id = ? OR t.created_by = ?
         ORDER BY t.id`), userID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	teams := make([]models.Team, 0)
	for rows.Next() {
		var t models.Team
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedBy, &t.CreatedAt); err != nil {
			return nil, err
		}
		teams = append(teams, t)
	}
	return teams, rows.Err()
}

// GetMembership возвращает роль пользователя в команде (sql.ErrNoRows, если не участник).
func (r *TeamRepo) GetMembership(ctx context.Context, teamID, userID uint) (string, error) {
	var role string
	err := r.db.QueryRowContext(ctx,
		r.dia.Ph(`SELECT role FROM team_members WHERE team_id = ? AND user_id = ?`), teamID, userID).
		Scan(&role)
	if err == sql.ErrNoRows {
		return "", sql.ErrNoRows
	}
	if err != nil {
		return "", err
	}
	return role, nil
}

// IsUserExists проверяет существование пользователя.
func (r *TeamRepo) IsUserExists(ctx context.Context, userID uint) (bool, error) {
	var one int
	err := r.db.QueryRowContext(ctx, r.dia.Ph(`SELECT 1 FROM users WHERE id = ?`), userID).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// Invite атомарно добавляет/обновляет участника команды. Роль owner здесь не выдается
// (проверка выполняется на уровне сервиса).
func (r *TeamRepo) Invite(ctx context.Context, teamID, userID uint, role string) error {
	var q string
	if r.dia.IsPostgres {
		q = `INSERT INTO team_members (team_id, user_id, role) VALUES ($1, $2, $3)
     ON CONFLICT (team_id, user_id) DO UPDATE SET role = EXCLUDED.role`
	} else {
		q = `INSERT INTO team_members (team_id, user_id, role) VALUES (?, ?, ?)
     ON DUPLICATE KEY UPDATE role = VALUES(role)`
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, q, teamID, userID, role); err != nil {
		return err
	}

	return tx.Commit()
}

// Members возвращает состав команды.
func (r *TeamRepo) Members(ctx context.Context, teamID uint) ([]models.TeamMemberWithUser, error) {
	orderRoleCase := `CASE tm.role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 ELSE 2 END`
	if !r.dia.IsPostgres {
		orderRoleCase = `FIELD(tm.role, 'owner', 'admin', 'member')`
	}
	query := r.dia.Ph(`SELECT tm.team_id, tm.user_id, tm.role, u.name, u.email
                 FROM team_members tm
                 JOIN users u ON u.id = tm.user_id
                 WHERE tm.team_id = ?
                 ORDER BY ` + orderRoleCase + `, tm.user_id`)

	rows, err := r.db.QueryContext(ctx, query, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := make([]models.TeamMemberWithUser, 0)
	for rows.Next() {
		var m models.TeamMemberWithUser
		if err := rows.Scan(&m.TeamID, &m.UserID, &m.Role, &m.UserName, &m.UserEmail); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}
