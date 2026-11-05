package repository

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"testovoe/internal/models"
)

// TaskFilter — параметры фильтрации списка задач.
type TaskFilter struct {
	TeamID     uint
	Status     string
	AssigneeID uint
	Limit      int // Макс. кол-во записей
    Offset     int // Сдвиг от начала
}

// TaskRepo — доступ к таблицам tasks, task_history, task_comments.
type TaskRepo struct {
	db  *sql.DB
	dia *Dialect
}

// TeamStatsResult — результат единого SQL-запроса статистики команды.
type TeamStatsResult struct {
    TotalTasks    int
    Todo          int
    InProgress    int
    Done          int
    AvgCloseSecs  sql.NullFloat64
    TotalComments int
    TopPerformers []TopPerformerRow
}

func NewTaskRepo(db *sql.DB) *TaskRepo {
	return &TaskRepo{db: db, dia: NewDialect(db)}
}

const taskColumns = `id, team_id, title, description, status, created_by, assignee_id, created_at, updated_at, closed_at, version`

func scanTask(scan func(dest ...interface{}) error) (*models.Task, error) {
	t := &models.Task{}
	err := scan(&t.ID, &t.TeamID, &t.Title, &t.Description, &t.Status,
		&t.CreatedBy, &t.AssigneeID, &t.CreatedAt, &t.UpdatedAt, &t.ClosedAt, &t.Version)
	if err == sql.ErrNoRows {
		return nil, sql.ErrNoRows
	}
	if err != nil {
		return nil, err
	}
	return t, nil
}

// Create вставляет задачу (version=1) и запись истории "created".
func (r *TaskRepo) Create(ctx context.Context, t *models.Task, history string) (*models.Task, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var id int64
	insert := r.dia.Ph(`INSERT INTO tasks (team_id, title, description, status, created_by, assignee_id, closed_at)
	                   VALUES (?, ?, ?, ?, ?, ?, ?)`) + r.dia.ReturningID()
	if r.dia.IsPostgres {
		err = tx.QueryRowContext(ctx, insert,
			t.TeamID, t.Title, t.Description, t.Status, t.CreatedBy, t.AssigneeID, t.ClosedAt).Scan(&id)
	} else {
		var res sql.Result
		res, err = tx.ExecContext(ctx, insert, t.TeamID, t.Title, t.Description, t.Status, t.CreatedBy, t.AssigneeID, t.ClosedAt)
		if err == nil {
			id, err = res.LastInsertId()
		}
	}
	if err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx,
		r.dia.Ph(`INSERT INTO task_history (task_id, changed_by, changes) VALUES (?, ?, ?)`),
		id, t.CreatedBy, history); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, uint(id))
}

// GetByID возвращает задачу по id (sql.ErrNoRows, если нет).
func (r *TaskRepo) GetByID(ctx context.Context, id uint) (*models.Task, error) {
	return scanTask(r.db.QueryRowContext(ctx,
		r.dia.Ph(`SELECT `+taskColumns+` FROM tasks WHERE id = ?`), id).Scan)
}

// List возвращает задачи команды с фильтрацией; видит только участник команды.
func (r *TaskRepo) List(ctx context.Context, f TaskFilter, userID uint) ([]models.Task, error) {
	q := `SELECT t.id, t.team_id, t.title, t.description, t.status, t.created_by, t.assignee_id,
	      t.created_at, t.updated_at, t.closed_at, t.version
	      FROM tasks t
	      JOIN team_members tm ON tm.team_id = t.team_id AND tm.user_id = ?`
	args := []interface{}{userID}

	if f.TeamID != 0 {
		q += ` WHERE t.team_id = ?`
		args = append(args, f.TeamID)
	}
	if f.Status != "" {
		if f.TeamID == 0 {
			q += ` WHERE`
		} else {
			q += ` AND`
		}
		q += ` t.status = ?`
		args = append(args, f.Status)
	}
	if f.AssigneeID != 0 {
		switch {
		case f.TeamID == 0 && f.Status == "":
			q += ` WHERE`
		default:
			q += ` AND`
		}
		q += ` t.assignee_id = ?`
		args = append(args, f.AssigneeID)
	}
	q += ` ORDER BY t.id DESC`

	if f.Limit > 0 {
        q += ` LIMIT ?`
        args = append(args, f.Limit)
        
        if f.Offset > 0 {
            q += ` OFFSET ?`
            args = append(args, f.Offset)
        }
    }

	rows, err := r.db.QueryContext(ctx, r.dia.Ph(q), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := make([]models.Task, 0)
	for rows.Next() {
		var t models.Task
		if err := rows.Scan(&t.ID, &t.TeamID, &t.Title, &t.Description, &t.Status,
			&t.CreatedBy, &t.AssigneeID, &t.CreatedAt, &t.UpdatedAt, &t.ClosedAt, &t.Version); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// Count возвращает общее количество задач по фильтрам (для метаданных пагинации).
func (r *TaskRepo) Count(ctx context.Context, f TaskFilter, userID uint) (int, error) {
    q := `SELECT COUNT(*) FROM tasks t`
    args := []interface{}{}
    
    // Если не указан team_id, проверяем, что пользователь вообще видит эту команду
    if f.TeamID == 0 {
        q += ` JOIN team_members tm ON tm.team_id = t.team_id AND tm.user_id = ?`
        args = append(args, userID)
    }

    whereClauses := []string{}
    if f.TeamID != 0 {
        whereClauses = append(whereClauses, `t.team_id = ?`)
        args = append(args, f.TeamID)
    }
    if f.Status != "" {
        whereClauses = append(whereClauses, `t.status = ?`)
        args = append(args, f.Status)
    }
    if f.AssigneeID != 0 {
        whereClauses = append(whereClauses, `t.assignee_id = ?`)
        args = append(args, f.AssigneeID)
    }
    
    if len(whereClauses) > 0 {
        q += ` WHERE ` + strings.Join(whereClauses, " AND ")
    }

    var count int
    err := r.db.QueryRowContext(ctx, r.dia.Ph(q), args...).Scan(&count)
    return count, err
}

// UpdateWithHistory выполняет оптимистичное обновление (WHERE id=? AND version=?)
// и атомарно добавляет записи истории. Возвращает sql.ErrNoRows, если версия не совпала
// или задача недоступна пользователю.
func (r *TaskRepo) UpdateWithHistory(ctx context.Context, id uint, in UpdateInput, history []string) (*models.Task, error) {
    // Проверка доступа — только чтение, можно вне транзакции
    cur, err := r.getForUser(ctx, id, in.UserID)
    if err != nil {
        return nil, err
    }

    // Вычисляем новые значения полей
    title := cur.Title
    if in.Title != nil {
        title = *in.Title
    }
    
    description := cur.Description
    if in.Description != nil {
        description = *in.Description
    }
    
    status := cur.Status
    if in.Status != nil {
        status = *in.Status
    }
    
    assignee := cur.AssigneeID
    if in.SetAssignee {
        assignee = in.AssigneeID
    }
    
    closedAt := cur.ClosedAt
    switch {
    case status == "done" && cur.Status != "done":
        now := time.Now().UTC()
        closedAt = &now
    case status != "done" && cur.Status == "done":
        closedAt = nil
    }

    newVersion := cur.Version + 1
    
    // === НАЧИНАЕМ ТРАНЗАКЦИЮ ДЛЯ ВСЕХ ПИШУЩИХ ОПЕРАЦИЙ ===
    tx, err := r.db.BeginTx(ctx, nil)
    if err != nil {
        return nil, err
    }
    defer func() { _ = tx.Rollback() }()

    // 1. UPDATE задачи внутри транзакции
    set := []string{"title = ?", "description = ?", "status = ?", "assignee_id = ?", "closed_at = ?", "version = ?"}
    args := []interface{}{title, description, status, assignee, closedAt, newVersion}
    
    if !r.dia.IsPostgres {
        set = append(set, "updated_at = NOW()")
    }
    
    where := ` WHERE id = ? AND version = ?`
    args = append(args, id, cur.Version)

    res, err := tx.ExecContext(ctx, 
        r.dia.Ph(`UPDATE tasks SET `+strings.Join(set, ", ")+where), args...)
    if err != nil {
        return nil, err // откат через defer
    }

    affected, err := res.RowsAffected()
    if err != nil {
        return nil, err
    }
    if affected == 0 {
        return nil, sql.ErrNoRows // optimistic lock conflict
    }

    // 2. INSERT истории внутри ТОЙ ЖЕ транзакции
    if len(history) > 0 {
        for _, h := range history {
            if _, err := tx.ExecContext(ctx,
                r.dia.Ph(`INSERT INTO task_history (task_id, changed_by, changes) VALUES (?, ?, ?)`),
                id, in.UserID, h); err != nil {
                return nil, err // откат через defer
            }
        }
    }

    // 3. Коммитим всё атомарно
    if err := tx.Commit(); err != nil {
        return nil, err
    }

    return r.GetByID(ctx, id)
}

// UpdateInput — данные обновления; UserID используется для проверки видимости задачи.
type UpdateInput struct {
	UserID      uint
	Title       *string
	Description *string
	Status      *string
	AssigneeID  *uint
	SetAssignee bool // различает "не менять" и "снять исполнителя" (nil)
}

// getForUser возвращает задачу, только если пользователь — участник её команды.
func (r *TaskRepo) getForUser(ctx context.Context, id, userID uint) (*models.Task, error) {
	return scanTask(r.db.QueryRowContext(ctx,
		r.dia.Ph(`SELECT t.id, t.team_id, t.title, t.description, t.status, t.created_by, t.assignee_id,
		         t.created_at, t.updated_at, t.closed_at, t.version
		         FROM tasks t
		         JOIN team_members tm ON tm.team_id = t.team_id AND tm.user_id = ?
		         WHERE t.id = ?`), userID, id).Scan)
}

// History возвращает историю изменений задачи (участник команды).
func (r *TaskRepo) History(ctx context.Context, taskID, userID uint) ([]models.TaskHistory, error) {
	rows, err := r.db.QueryContext(ctx,
		r.dia.Ph(`SELECT th.id, th.task_id, th.changed_by, th.changes, th.created_at
		          FROM task_history th
		          JOIN tasks t ON t.id = th.task_id
		          JOIN team_members tm ON tm.team_id = t.team_id AND tm.user_id = ?
		          WHERE th.task_id = ?
		          ORDER BY th.id DESC`), userID, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]models.TaskHistory, 0)
	for rows.Next() {
		var h models.TaskHistory
		if err := rows.Scan(&h.ID, &h.TaskID, &h.ChangedBy, &h.Changes, &h.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, h)
	}
	return items, rows.Err()
}

// AddComment добавляет комментарий (только участник команды задачи).
func (r *TaskRepo) AddComment(ctx context.Context, taskID, userID uint, content string) (*models.TaskComment, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var one int
	err = tx.QueryRowContext(ctx,
		r.dia.Ph(`SELECT 1 FROM tasks t
		          JOIN team_members tm ON tm.team_id = t.team_id AND tm.user_id = ?
		          WHERE t.id = ?`), userID, taskID).Scan(&one)
	if err == sql.ErrNoRows {
		return nil, sql.ErrNoRows
	}
	if err != nil {
		return nil, err
	}

	var id int64
	ins := r.dia.Ph(`INSERT INTO task_comments (task_id, user_id, content) VALUES (?, ?, ?)`) + r.dia.ReturningID()
	if r.dia.IsPostgres {
		err = tx.QueryRowContext(ctx, ins, taskID, userID, content).Scan(&id)
	} else {
		var res sql.Result
		res, err = tx.ExecContext(ctx, ins, taskID, userID, content)
		if err == nil {
			id, err = res.LastInsertId()
		}
	}
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	c := &models.TaskComment{ID: uint(id), TaskID: taskID, UserID: userID, Content: content}
	err = r.db.QueryRowContext(ctx,
		r.dia.Ph(`SELECT created_at FROM task_comments WHERE id = ?`), id).Scan(&c.CreatedAt)
	return c, err
}

// Comments возвращает комментарии задачи (участник команды).
func (r *TaskRepo) Comments(ctx context.Context, taskID, userID uint) ([]models.TaskComment, error) {
	rows, err := r.db.QueryContext(ctx,
		r.dia.Ph(`SELECT tc.id, tc.task_id, tc.user_id, tc.content, tc.created_at
		          FROM task_comments tc
		          JOIN tasks t ON t.id = tc.task_id
		          JOIN team_members tm ON tm.team_id = t.team_id AND tm.user_id = ?
		          WHERE tc.task_id = ?
		          ORDER BY tc.id ASC`), userID, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]models.TaskComment, 0)
	for rows.Next() {
		var c models.TaskComment
		if err := rows.Scan(&c.ID, &c.TaskID, &c.UserID, &c.Content, &c.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

// Stats возвращает полную статистику команды одним CTE-запросом.
// Метрики: задачи по статусам, топ-3 исполнителя по закрытым задачам за 30 дней,
// среднее время закрытия, количество комментариев.
func (r *TaskRepo) Stats(ctx context.Context, teamID uint) (*TeamStatsResult, error) {
    // Разные диалекты для вычисления среднего времени закрытия и фильтра по датам
    var avgExpr, dateFilter string
    if r.dia.IsPostgres {
        avgExpr = `AVG(CASE WHEN status = 'done' AND closed_at IS NOT NULL 
            THEN EXTRACT(EPOCH FROM (closed_at - created_at)) END)`
        dateFilter = `t.closed_at >= NOW() - INTERVAL '30 days'`
    } else {
        avgExpr = `AVG(CASE WHEN status = 'done' AND closed_at IS NOT NULL 
            THEN TIMESTAMPDIFF(SECOND, created_at, closed_at) END)`
        dateFilter = `t.closed_at >= NOW() - INTERVAL 30 DAY`
    }

    // Один CTE-запрос, объединяющий все метрики
    query := `
WITH task_stats AS (
    SELECT 
        COUNT(*) AS total_tasks,
        COALESCE(SUM(CASE WHEN status = 'todo' THEN 1 ELSE 0 END), 0) AS todo,
        COALESCE(SUM(CASE WHEN status = 'in_progress' THEN 1 ELSE 0 END), 0) AS in_progress,
        COALESCE(SUM(CASE WHEN status = 'done' THEN 1 ELSE 0 END), 0) AS done,
        COALESCE(` + avgExpr + `, 0) AS avg_close_secs
    FROM tasks 
    WHERE team_id = ?
),
comment_stats AS (
    SELECT COUNT(*) AS total_comments
    FROM task_comments tc
    JOIN tasks t ON t.id = tc.task_id
    WHERE t.team_id = ?
),
top_performers AS (
    SELECT 
        tm.user_id, 
        u.name,
        COUNT(t.id) AS done_cnt
    FROM team_members tm
    JOIN users u ON u.id = tm.user_id
    LEFT JOIN tasks t ON t.team_id = tm.team_id 
        AND t.assignee_id = tm.user_id 
        AND t.status = 'done' 
        AND ` + dateFilter + `
    WHERE tm.team_id = ?
    GROUP BY tm.user_id, u.name
    ORDER BY done_cnt DESC, tm.user_id ASC
    LIMIT 3
)
SELECT 
    ts.total_tasks, 
    ts.todo, 
    ts.in_progress, 
    ts.done, 
    ts.avg_close_secs,
    cs.total_comments,
    tp.user_id AS performer_id, 
    tp.name AS performer_name, 
    tp.done_cnt AS performer_done_cnt
FROM task_stats ts
CROSS JOIN comment_stats cs
LEFT JOIN top_performers tp ON TRUE
`

    rows, err := r.db.QueryContext(ctx, r.dia.Ph(query), teamID, teamID, teamID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    result := &TeamStatsResult{
        TopPerformers: make([]TopPerformerRow, 0),
    }

    firstRow := true
    for rows.Next() {
        var (
            totalTasks    int
            todo          int
            inProgress    int
            done          int
            avgCloseSecs  sql.NullFloat64
            totalComments int
            performerID   sql.NullInt64
            performerName sql.NullString
            performerCnt  sql.NullInt64
        )

        if err := rows.Scan(
            &totalTasks, &todo, &inProgress, &done, &avgCloseSecs,
            &totalComments, &performerID, &performerName, &performerCnt,
        ); err != nil {
            return nil, err
        }

        // Агрегаты берем из первой строки (они одинаковые во всех строках)
        if firstRow {
            result.TotalTasks = totalTasks
            result.Todo = todo
            result.InProgress = inProgress
            result.Done = done
            result.AvgCloseSecs = avgCloseSecs
            result.TotalComments = totalComments
            firstRow = false
        }

        // Топ-исполнителей собираем из всех строк
        if performerID.Valid {
            result.TopPerformers = append(result.TopPerformers, TopPerformerRow{
                UserID:   uint(performerID.Int64),
                UserName: performerName.String,
                DoneCnt:  int(performerCnt.Int64),
            })
        }
    }

    if err := rows.Err(); err != nil {
        return nil, err
    }

    return result, nil
}

// TopPerformerRow — строка рейтинга исполнителей.
type TopPerformerRow struct {
	UserID   uint   `db:"user_id"`
	UserName string `db:"name"`
	DoneCnt  int    `db:"done_cnt"`
}

// EnsureMemberOfTaskTeam проверяет участие пользователя в команде задачи;
// возвращает TeamID задачи. Ошибки: sql.ErrNoRows — задача не найдена или нет доступа.
func (r *TaskRepo) EnsureMemberOfTaskTeam(ctx context.Context, taskID, userID uint) (uint, error) {
	var teamID uint
	err := r.db.QueryRowContext(ctx,
		r.dia.Ph(`SELECT t.team_id FROM tasks t
		          JOIN team_members tm ON tm.team_id = t.team_id AND tm.user_id = ?
		          WHERE t.id = ?`), userID, taskID).Scan(&teamID)
	if err == sql.ErrNoRows {
		return 0, sql.ErrNoRows
	}
	return teamID, err
}

// GetVisible возвращает задачу, только если пользователь — участник её команды.
func (r *TaskRepo) GetVisible(ctx context.Context, id, userID uint) (*models.Task, error) {
	return r.getForUser(ctx, id, userID)
}
