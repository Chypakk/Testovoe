package service

import (
	"testing"

	"testovoe/internal/models"
)

func uintPtr(v uint) *uint { return &v }

func TestResolveTaskRights(t *testing.T) {
	const (
		ownerID    = uint(1) // owner команды, не участник задачи
		adminID    = uint(2) // admin команды
		creatorID  = uint(3) // создатель задачи
		assigneeID = uint(4) // исполнитель задачи
		memberID   = uint(5) // простой участник команды
		userU      = uint(6) // пользователь вне команды
	)

	task := func(mut func(*models.Task)) *models.Task {
		tsk := &models.Task{ID: 10, TeamID: 100, CreatedBy: creatorID, AssigneeID: uintPtr(assigneeID)}
		if mut != nil {
			mut(tsk)
		}
		return tsk
	}

	tests := []struct {
		name       string
		role       string
		task       *models.Task
		userID     uint
		wantAll    bool
		wantTitle  bool
		wantDesc   bool
		wantStatus bool
		wantReasgn bool
	}{
		{"owner команды — полный доступ к чужой задаче", RoleOwner, task(nil), ownerID, true, true, true, true, true},
		{"admin команды — полный доступ к чужой задаче", RoleAdmin, task(nil), adminID, true, true, true, true, true},
		{"создатель задачи (member) — все поля, без canEditAll", RoleMember, task(nil), creatorID, false, true, true, true, true},
		{"исполнитель (member) — только статус", RoleMember, task(nil), assigneeID, false, false, false, true, false},
		{"сторонний member — только чтение", RoleMember, task(nil), memberID, false, false, false, false, false},
		{"не-участник (role=\"\") — доступа нет", "", task(nil), userU, false, false, false, false, false},
		{"создатель без исполнителя — может назначить", RoleMember, task(func(m *models.Task) { m.AssigneeID = nil }), creatorID, false, true, true, true, true},
		{"owner он же создатель — полный доступ", RoleOwner, task(func(m *models.Task) { m.CreatedBy = ownerID }), ownerID, true, true, true, true, true},
		{"member он же и создатель, и исполнитель — права создателя", RoleMember, task(func(m *models.Task) { m.CreatedBy = assigneeID }), assigneeID, false, true, true, true, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := resolveTaskRights(tc.role, tc.task, tc.userID)
			if r.canEditAll != tc.wantAll {
				t.Errorf("canEditAll = %v, want %v", r.canEditAll, tc.wantAll)
			}
			if r.canChangeTitle != tc.wantTitle {
				t.Errorf("canChangeTitle = %v, want %v", r.canChangeTitle, tc.wantTitle)
			}
			if r.canChangeDesc != tc.wantDesc {
				t.Errorf("canChangeDesc = %v, want %v", r.canChangeDesc, tc.wantDesc)
			}
			if r.canChangeStat != tc.wantStatus {
				t.Errorf("canChangeStat = %v, want %v", r.canChangeStat, tc.wantStatus)
			}
			if r.canReassign != tc.wantReasgn {
				t.Errorf("canReassign = %v, want %v", r.canReassign, tc.wantReasgn)
			}
		})
	}
}

// Проверок на «разные sentinel-ошибки» недостаточно для проверки порядка —
// порядок (доступ → права → версия) покрыт тестами TestUpdateAccessAndVersionChecks.
