DROP TRIGGER IF EXISTS trg_tasks_updated_at ON tasks;
DROP TRIGGER IF EXISTS trg_team_members_updated_at ON team_members;
DROP TRIGGER IF EXISTS trg_teams_updated_at ON teams;
DROP TRIGGER IF EXISTS trg_users_updated_at ON users;

DROP FUNCTION IF EXISTS set_updated_at();

DROP TABLE IF EXISTS task_comments;
DROP TABLE IF EXISTS task_history;
DROP TABLE IF EXISTS tasks;
DROP TABLE IF EXISTS team_members;
DROP TABLE IF EXISTS teams;
DROP TABLE IF EXISTS users;