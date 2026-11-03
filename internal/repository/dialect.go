package repository

import (
	"database/sql"
	"strconv"
	"strings"
)

// Dialect — адаптация SQL под конкретную СУБД.
// Проект поддерживает MySQL и PostgreSQL (см. DB_DRIVER в конфиге).
type Dialect struct {
	IsPostgres bool
}

// NewDialectFromDriver определяет диалект по имени драйвера database/sql ("pgx"/"postgres"/"mysql").
func NewDialectFromDriver(driverName string) *Dialect {
	name := strings.ToLower(strings.TrimSpace(driverName))
	return &Dialect{IsPostgres: strings.Contains(name, "pgx") || strings.Contains(name, "postgres")}
}

// NewDialect определяет диалект по драйверу открытого соединения.
// pgx/v5 stdlib реализует NamedDriver и сообщает имя "pgx"; остальные драйверы считаем MySQL-совместимыми.
func NewDialect(db *sql.DB) *Dialect {
	if dn, ok := db.Driver().(interface{ Name() string }); ok {
		return NewDialectFromDriver(dn.Name())
	}
	return &Dialect{IsPostgres: false}
}

// Ph превращает "?" в "$N" для postgres; для mysql возвращает как есть.
func (d *Dialect) Ph(query string) string {
	if !d.IsPostgres {
		return query
	}
	n := 0
	var b strings.Builder
	for _, ch := range query {
		if ch == '?' {
			n++
			b.WriteByte('$')
			b.WriteString(strconv.Itoa(n))
		} else {
			b.WriteRune(ch)
		}
	}
	return b.String()
}

// ReturningID — суффикс RETURNING id для postgres-вставок.
func (d *Dialect) ReturningID() string {
	if d.IsPostgres {
		return " RETURNING id"
	}
	return ""
}
