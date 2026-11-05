package repository

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
)

func TestTeamStats(t *testing.T) {
	ctx := context.Background()

	mysqlContainer, err := mysql.Run(ctx, "mysql:8.0",
		mysql.WithDatabase("testovoe"),
		mysql.WithUsername("root"),
		mysql.WithPassword("password"),
	)
	if err != nil {
		t.Skipf("docker недоступен, пропускаем интеграционный тест: %v", err)
	}
	defer func() { _ = testcontainers.TerminateContainer(mysqlContainer) }()

	host, _ := mysqlContainer.Host(ctx)
	port, _ := mysqlContainer.MappedPort(ctx, "3306/tcp")
	dsn := fmt.Sprintf("root:password@tcp(%s:%s)/testovoe?multiStatements=true&parseTime=true", host, port)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	// Накатываем твою up-миграцию. Путь — от пакета теста до корня.
	m, err := migrate.New("file://../../migrations/mysql", dsn)
	if err != nil {
		t.Fatalf("migrate new: %v", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		t.Fatalf("migrate up: %v", err)
	}

	seedStatsFixtures(t, db)

	repo := NewTaskRepo(db)
	stats, err := repo.Stats(ctx, 1 /* teamA */, 3)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}

	// teamA: 2 todo + 1 in_progress + 2 done = 5 задач.
	// В teamB лежит ещё 5 — они НЕ должны попасть в отчёт по teamA.
	assertInt(t, "total tasks", stats.TotalTasks, 5)
	assertInt(t, "todo", stats.Todo, 2)
	assertInt(t, "in_progress", stats.InProgress, 1)
	assertInt(t, "done", stats.Done, 2)

	// Комментарии: 3 в задачах teamA, 2 в teamB -> в отчёте только 3.
	assertInt(t, "total comments", stats.TotalComments, 3)

	// Среднее время закрытия по done-задачам teamA должно считаться (> 0).
	if !stats.AvgCloseSecs.Valid || stats.AvgCloseSecs.Float64 <= 0 {
		t.Fatalf("avg close secs expected > 0, got %+v", stats.AvgCloseSecs)
	}

	// Топ-3: не больше трёх, и все исполнители — из teamA (не нулевые id).
	if len(stats.TopPerformers) == 0 || len(stats.TopPerformers) > 3 {
		t.Fatalf("top performers count out of range: %d", len(stats.TopPerformers))
	}
	for _, p := range stats.TopPerformers {
		if p.UserID == 0 {
			t.Fatalf("top performer with zero user id: %+v", p)
		}
	}

	// Изоляция команд: отчёт по пустной/чужой команде не должен видеть данные teamA.
	empty, err := repo.Stats(ctx, 999, 3)
	if err != nil {
		t.Fatalf("Stats empty team: %v", err)
	}
	assertInt(t, "empty team total", empty.TotalTasks, 0)
	assertInt(t, "empty team comments", empty.TotalComments, 0)
}

func seedStatsFixtures(t *testing.T, db *sql.DB) {
	t.Helper()
	now := time.Now().UTC()
	mk := func(d time.Duration) time.Time { return now.Add(-d) }

	// users: 1,2,3 — члены teamA; 4 — член teamB; 5 — исполнитель вне топ-окна.
	mustExec(t, db, `INSERT INTO users (id, email, password_hash, name) VALUES
		(1,'a1@x.com','h','A1'),(2,'a2@x.com','h','A2'),(3,'a3@x.com','h','A3'),
		(4,'b1@x.com','h','B1'),(5,'a5@x.com','h','A5')`)

	// teams: 1 = teamA (создатель 1), 2 = teamB (создатель 4).
	mustExec(t, db, `INSERT INTO teams (id, name, created_by) VALUES
		(1,'TeamA',1),(2,'TeamB',4)`)

	// участники teamA: owner=1, admin=2, member=3; teamB: owner=4.
	mustExec(t, db, `INSERT INTO team_members (team_id, user_id, role) VALUES
		(1,1,'owner'),(1,2,'admin'),(1,3,'member'),(2,4,'owner')`)

	// tasks teamA: 2 todo, 1 in_progress, 2 done (одна закрыта 10 дней назад,
	// вторая 40 дней назад — вне 30-дневного окна для топа, но в среднем).
	mustExec(t, db, `INSERT INTO tasks
		(id, team_id, title, description, status, created_by, assignee_id, created_at, updated_at, closed_at, version) VALUES
		(101,1,'t1','', 'todo',       1, 2, ?, ?, NULL, 1),
		(102,1,'t2','', 'todo',       1, 3, ?, ?, NULL, 1),
		(103,1,'t3','', 'in_progress',1, 2, ?, ?, NULL, 1),
		(104,1,'t4','', 'done',       1, 2, ?, ?, ?,    1),
		(105,1,'t5','', 'done',       1, 3, ?, ?, ?,    1)`,
		mk(20*time.Hour), mk(1*time.Hour), // 101
		mk(20*time.Hour), mk(1*time.Hour), // 102
		mk(20*time.Hour), mk(1*time.Hour), // 103
		mk(15*24*time.Hour), mk(14*24*time.Hour), mk(5*24*time.Hour),  // 104 closed 5d ago
		mk(45*24*time.Hour), mk(44*24*time.Hour), mk(40*24*time.Hour), // 105 closed 40d ago
	)

	// tasks teamB — должны игнорироваться отчётом по teamA.
	mustExec(t, db, `INSERT INTO tasks
		(id, team_id, title, description, status, created_by, assignee_id, created_at, updated_at, closed_at, version) VALUES
		(201,2,'b1','','todo',4,NULL,?,?,NULL,1),
		(202,2,'b2','','todo',4,NULL,?,?,NULL,1),
		(203,2,'b3','','done',4,  4,?,?,?,1),
		(204,2,'b4','','done',4,  4,?,?,?,1),
		(205,2,'b5','','done',4,  4,?,?,?,1)`,
		mk(time.Hour), mk(time.Minute),
		mk(time.Hour), mk(time.Minute),
		mk(2*time.Hour), mk(time.Hour), mk(30*time.Minute),
		mk(2*time.Hour), mk(time.Hour), mk(30*time.Minute),
		mk(2*time.Hour), mk(time.Hour), mk(30*time.Minute),
	)

	// comments: 3 в задачах teamA, 2 в teamB.
	mustExec(t, db, `INSERT INTO task_comments (id, task_id, user_id, content) VALUES
		(1,101,2,'c'),(2,103,3,'c'),(3,104,1,'c'),
		(4,203,4,'c'),(5,204,4,'c')`)
}

func mustExec(t *testing.T, db *sql.DB, q string, args ...any) {
	t.Helper()
	if _, err := db.Exec(q, args...); err != nil {
		t.Fatalf("exec %q: %v", q, err)
	}
}

func assertInt(t *testing.T, name string, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %d, want %d", name, got, want)
	}
}