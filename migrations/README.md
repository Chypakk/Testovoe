# Database Migrations

This directory contains database migrations for the project using golang-migrate/migrate.

## Migration Files

1. **000001_create_users_table.up/down.sql**
   - Creates/drops the `users` table
   - Contains fields: id, email, password_hash, name, created_at, updated_at

2. **000002_create_teams_table.up/down.sql**
   - Creates/drops the `teams` table
   - Contains fields: id, name, created_by, created_at, updated_at
   - Foreign key reference to users table

3. **000003_create_team_members_table.up/down.sql**
   - Creates/drops the `team_members` table
   - Contains fields: id, team_id, user_id, role, joined_at, updated_at
   - Foreign key references to teams and users tables
   - Unique constraint on (team_id, user_id) combination

## Running Migrations

To apply migrations:
```bash
migrate -path ./migrations -database "postgres://user:password@localhost:5432/testovoe?sslmode=disable" up
```

To roll back migrations:
```bash
migrate -path ./migrations -database "postgres://user:password@localhost:5432/testovoe?sslmode=disable" down
```

To create a new migration:
```bash
migrate create -ext sql -dir ./migrations -seq migration_name
```

## Database Schema

The database consists of three main tables:
- `users` - User accounts
- `teams` - Team information
- `team_members` - Team membership with roles