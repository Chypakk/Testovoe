package service

import (
	"context"
	"database/sql"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"testovoe/internal/cache"
	"testovoe/internal/models"
	"testovoe/internal/repository"
)

// newMockServices создаёт TaskService поверх sqlmock: репозитории работают с
// *sql.DB как с обычной БД, запросы перехватываются и эмулируются на лету.
func newMockServices(t *testing.T) (*TaskService, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	return NewTaskService(repository.NewTaskRepo(db), repository.NewTeamRepo(db), repository.NewUserRepo(db), cache.NewInMemoryCache()),
		mock,
		func() { _ = db.Close() }
}

func taskRow(id, teamID uint, title, status string, createdBy uint, assignee any, version int) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "team_id", "title", "description", "status",
		"created_by", "assignee_id", "created_at", "updated_at", "closed_at", "version"}).
		AddRow(id, teamID, title, "", status, createdBy, assignee, time.Now(), time.Now(), nil, version)
}

// expectMembership эмулирует SELECT role FROM team_members WHERE team_id=? AND user_id=?
func expectMembership(mock sqlmock.Sqlmock, teamID, userID uint, role string) {
	mock.ExpectQuery(regexp.QuoteMeta("SELECT role FROM team_members")).
		WithArgs(teamID, userID).
		WillReturnRows(sqlmock.NewRows([]string{"role"}).AddRow(role))
}

func expectNoMembership(mock sqlmock.Sqlmock, teamID, userID uint) {
	mock.ExpectQuery(regexp.QuoteMeta("SELECT role FROM team_members")).
		WithArgs(teamID, userID).
		WillReturnError(sql.ErrNoRows)
}

func strPtr(s string) *string { return &s }

// helper для сравнения ошибок
func wantErr(t *testing.T, got error, want error) {
	t.Helper()
	if got != want {
		t.Fatalf("err = %v, want %v", got, want)
	}
}

func TestUpdateRightsMatrix(t *testing.T) {
	const (
		ownerID    = uint(1)
		adminID    = uint(2)
		creatorID  = uint(3)
		assigneeID = uint(4)
		memberID   = uint(5)
		taskID     = uint(10)
		teamID     = uint(100)
		newAssign  = uint(7)
	)

	type step struct {
		name string
		req  models.UpdateTaskRequest
		err  error // ожидаемая ошибка; nil — успех
	}

	tests := []struct {
		name       string
		userID     uint
		steps      []step
	}{
		{
			name:   "owner команды может менять всё в чужой задаче",
			userID: ownerID,
			steps: []step{
				{"title", models.UpdateTaskRequest{Title: "new title"}, nil},
				{"description", models.UpdateTaskRequest{Description: "desc"}, nil},
				{"status", models.UpdateTaskRequest{Status: strPtr("in_progress")}, nil},
				{"assignee", models.UpdateTaskRequest{AssigneeID: uintPtr(newAssign)}, nil},
			},
		},
		{
			name:   "admin команды может менять всё",
			userID: adminID,
			steps: []step{
				{"assignee", models.UpdateTaskRequest{AssigneeID: uintPtr(newAssign)}, nil},
			},
		},
		{
			name:   "создатель (member) меняет все поля своей задачи",
			userID: creatorID,
			steps: []step{
				{"title", models.UpdateTaskRequest{Title: "t2"}, nil},
				{"assignee", models.UpdateTaskRequest{AssigneeID: uintPtr(newAssign)}, nil},
				{"unassign", models.UpdateTaskRequest{AssigneeID: uintPtr(0)}, nil},
			},
		},
		{
			name:   "исполнитель меняет только статус",
			userID: assigneeID,
			steps: []step{
				{"status — ок", models.UpdateTaskRequest{Status: strPtr("done")}, nil},
				{"title — 403", models.UpdateTaskRequest{Title: "hack"}, ErrForbidden},
				{"description — 403", models.UpdateTaskRequest{Description: "hack"}, ErrForbidden},
				{"assignee — 403", models.UpdateTaskRequest{AssigneeID: uintPtr(newAssign)}, ErrForbidden},
			},
		},
		{
			name:   "сторонний member — только чтение",
			userID: memberID,
			steps: []step{
				{"title — 403", models.UpdateTaskRequest{Title: "x"}, ErrForbidden},
				{"status — 403", models.UpdateTaskRequest{Status: strPtr("done")}, ErrForbidden},
				{"assignee — 403", models.UpdateTaskRequest{AssigneeID: uintPtr(memberID)}, ErrForbidden},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, mock, done := newMockServices(t)
			defer done()
			ctx := context.Background()

			roleByUser := map[uint]string{ownerID: RoleOwner, adminID: RoleAdmin, creatorID: RoleMember, assigneeID: RoleMember, memberID: RoleMember}

			for _, st := range tc.steps {
				// GetVisible → getForUser
				mock.ExpectQuery(regexp.QuoteMeta("FROM tasks t")).
					WithArgs(tc.userID, taskID).
					WillReturnRows(taskRow(taskID, teamID, "orig", "todo", creatorID, assigneeID, 1))
				// роль пользователя в команде
				expectMembership(mock, teamID, tc.userID, roleByUser[tc.userID])

				switch st.err {
				case ErrForbidden:
					_, err := svc.Update(ctx, taskID, tc.userID, st.req, 0)
					wantErr(t, err, ErrForbidden)
				case nil:
					// успешный путь: если меняется assignee — проверка его членства,
					// затем UPDATE + история + перечитка
					if st.req.AssigneeID != nil && *st.req.AssigneeID != 0 {
						expectMembership(mock, teamID, *st.req.AssigneeID, RoleMember)
					}
					mock.ExpectExec(regexp.QuoteMeta("UPDATE tasks SET")).
						WillReturnResult(sqlmock.NewResult(0, 1))
					mock.ExpectBegin()
					mock.ExpectExec(regexp.QuoteMeta("INSERT INTO task_history")).
						WillReturnResult(sqlmock.NewResult(1, 1))
					mock.ExpectCommit()
					mock.ExpectQuery(regexp.QuoteMeta("FROM tasks WHERE id = ?")).
						WithArgs(taskID).
						WillReturnRows(taskRow(taskID, teamID, "orig", "todo", creatorID, assigneeID, 2))

					got, err := svc.Update(ctx, taskID, tc.userID, st.req, 0)
					if err != nil {
						t.Fatalf("%s: unexpected err %v", st.name, err)
					}
					if got == nil || got.ID != taskID {
						t.Fatalf("%s: bad result %+v", st.name, got)
					}
				default:
					t.Fatalf("bad step expectation")
				}
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unmet expectations: %v", err)
			}
		})
	}
}

func TestUpdateAccessAndVersionChecks(t *testing.T) {
	const (
		creatorID  = uint(3)
		assigneeID = uint(4)
		taskID     = uint(10)
		teamID     = uint(100)
	)
	ctx := context.Background()

	t.Run("не-участник не видит задачу — 404", func(t *testing.T) {
		svc, mock, done := newMockServices(t)
		defer done()
		mock.ExpectQuery(regexp.QuoteMeta("FROM tasks t")).
			WithArgs(uint(99), taskID).
			WillReturnError(sql.ErrNoRows)
		_, err := svc.Update(ctx, taskID, 99, models.UpdateTaskRequest{Title: "x"}, 0)
		wantErr(t, err, ErrTaskNotFound)
	})

	t.Run("права проверяются до версии (403 важнее 409)", func(t *testing.T) {
		svc, mock, done := newMockServices(t)
		defer done()
		mock.ExpectQuery(regexp.QuoteMeta("FROM tasks t")).
			WithArgs(assigneeID, taskID).
			WillReturnRows(taskRow(taskID, teamID, "orig", "todo", creatorID, assigneeID, 5))
		expectMembership(mock, teamID, assigneeID, RoleMember)
		_, err := svc.Update(ctx, taskID, assigneeID, models.UpdateTaskRequest{Title: "nope"}, 1 /* устаревшая версия */)
		wantErr(t, err, ErrForbidden)
	})

	t.Run("версия проверяется после прав, до записи (409)", func(t *testing.T) {
		svc, mock, done := newMockServices(t)
		defer done()
		mock.ExpectQuery(regexp.QuoteMeta("FROM tasks t")).
			WithArgs(creatorID, taskID).
			WillReturnRows(taskRow(taskID, teamID, "orig", "todo", creatorID, assigneeID, 5))
		expectMembership(mock, teamID, creatorID, RoleMember)
		_, err := svc.Update(ctx, taskID, creatorID, models.UpdateTaskRequest{Title: "new"}, 4)
		wantErr(t, err, ErrVersionConflict)
	})

	t.Run("гонка при записи — 409", func(t *testing.T) {
		svc, mock, done := newMockServices(t)
		defer done()
		mock.ExpectQuery(regexp.QuoteMeta("FROM tasks t")).
			WithArgs(creatorID, taskID).
			WillReturnRows(taskRow(taskID, teamID, "orig", "todo", creatorID, assigneeID, 5))
		expectMembership(mock, teamID, creatorID, RoleMember)
		mock.ExpectExec(regexp.QuoteMeta("UPDATE tasks SET")).
			WillReturnResult(sqlmock.NewResult(0, 0)) // affected 0 — кто-то обновил раньше
		_, err := svc.Update(ctx, taskID, creatorID, models.UpdateTaskRequest{Title: "new"}, 0)
		wantErr(t, err, ErrVersionConflict)
	})

	t.Run("назначение не-участника команды — 400", func(t *testing.T) {
		svc, mock, done := newMockServices(t)
		defer done()
		outsider := uint(42)
		mock.ExpectQuery(regexp.QuoteMeta("FROM tasks t")).
			WithArgs(creatorID, taskID).
			WillReturnRows(taskRow(taskID, teamID, "orig", "todo", creatorID, assigneeID, 1))
		expectMembership(mock, teamID, creatorID, RoleMember)
		expectNoMembership(mock, teamID, outsider)
		_, err := svc.Update(ctx, taskID, creatorID, models.UpdateTaskRequest{AssigneeID: &outsider}, 0)
		wantErr(t, err, ErrInvalidInput)
	})

	t.Run("невалидный статус — 400", func(t *testing.T) {
		svc, mock, done := newMockServices(t)
		defer done()
		mock.ExpectQuery(regexp.QuoteMeta("FROM tasks t")).
			WithArgs(creatorID, taskID).
			WillReturnRows(taskRow(taskID, teamID, "orig", "todo", creatorID, assigneeID, 1))
		expectMembership(mock, teamID, creatorID, RoleMember)
		_, err := svc.Update(ctx, taskID, creatorID, models.UpdateTaskRequest{Status: strPtr("archived")}, 0)
		wantErr(t, err, ErrInvalidInput)
	})

	t.Run("пустой патч без изменений — задача возвращается без записи", func(t *testing.T) {
		svc, mock, done := newMockServices(t)
		defer done()
		mock.ExpectQuery(regexp.QuoteMeta("FROM tasks t")).
			WithArgs(creatorID, taskID).
			WillReturnRows(taskRow(taskID, teamID, "orig", "todo", creatorID, assigneeID, 1))
		expectMembership(mock, teamID, creatorID, RoleMember)
		got, err := svc.Update(ctx, taskID, creatorID, models.UpdateTaskRequest{Title: "orig"}, 0)
		if err != nil {
			t.Fatalf("unexpected err %v", err)
		}
		if !strings.EqualFold(got.Title, "orig") {
			t.Fatalf("title = %q", got.Title)
		}
	})
}

func TestStatsRoleRestriction(t *testing.T) {
	const (
		ownerID  = uint(1)
		adminID  = uint(2)
		memberID = uint(5)
		stranger = uint(9)
		teamID   = uint(100)
	)
	ctx := context.Background()

	t.Run("member — 403", func(t *testing.T) {
		svc, mock, done := newMockServices(t)
		defer done()
		expectMembership(mock, teamID, memberID, RoleMember)
		_, err := svc.Stats(ctx, teamID, memberID)
		wantErr(t, err, ErrForbidden)
	})

	t.Run("не-участник — 403 без раскрытия команды", func(t *testing.T) {
		svc, mock, done := newMockServices(t)
		defer done()
		expectNoMembership(mock, teamID, stranger)
		_, err := svc.Stats(ctx, teamID, stranger)
		wantErr(t, err, ErrForbidden)
	})

	t.Run("owner — успешно", func(t *testing.T) {
		svc, mock, done := newMockServices(t)
		defer done()
		expectMembership(mock, teamID, ownerID, RoleOwner)
		mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*)")).
			WithArgs(teamID).
			WillReturnRows(sqlmock.NewRows([]string{"total", "todo", "in_progress", "done", "avg", "comments"}).
				AddRow(10, 4, 3, 3, sql.NullFloat64{Float64: 3600, Valid: true}, 0))
		mock.ExpectQuery(regexp.QuoteMeta("FROM task_comments")).
			WithArgs(teamID).
			WillReturnRows(sqlmock.NewRows([]string{"cnt"}).AddRow(7))
		mock.ExpectQuery(regexp.QuoteMeta("COALESCE(SUM(CASE WHEN t.status = 'done'")).
			WithArgs(teamID, 5).
			WillReturnRows(sqlmock.NewRows([]string{"user_id", "name", "done_cnt"}).
				AddRow(ownerID, "Ann", 3))

		stats, err := svc.Stats(ctx, teamID, ownerID)
		if err != nil {
			t.Fatalf("unexpected err %v", err)
		}
		if stats.TotalTasks != 10 || stats.TotalComments != 7 || len(stats.TopPerformers) != 1 {
			t.Fatalf("bad stats: %+v", stats)
		}
	})

	t.Run("admin — успешно", func(t *testing.T) {
		svc, mock, done := newMockServices(t)
		defer done()
		expectMembership(mock, teamID, adminID, RoleAdmin)
		mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*)")).
			WithArgs(teamID).
			WillReturnRows(sqlmock.NewRows([]string{"total", "todo", "in_progress", "done", "avg", "comments"}).
				AddRow(0, 0, 0, 0, sql.NullFloat64{}, 0))
		mock.ExpectQuery(regexp.QuoteMeta("FROM task_comments")).
			WithArgs(teamID).
			WillReturnRows(sqlmock.NewRows([]string{"cnt"}).AddRow(0))
		mock.ExpectQuery(regexp.QuoteMeta("COALESCE(SUM(CASE WHEN t.status = 'done'")).
			WithArgs(teamID, 5).
			WillReturnRows(sqlmock.NewRows([]string{"user_id", "name", "done_cnt"}))

		stats, err := svc.Stats(ctx, teamID, adminID)
		if err != nil {
			t.Fatalf("unexpected err %v", err)
		}
		if stats.TopPerformers == nil || len(stats.TopPerformers) != 0 {
			t.Fatalf("TopPerformers should be empty non-nil slice: %+v", stats.TopPerformers)
		}
	})
}
