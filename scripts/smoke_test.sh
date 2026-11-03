#!/usr/bin/env bash
# Скрипт ручного smoke-тестирования API.
# Предварительно: docker compose up -d postgres redis && go run cmd/server/main.go
BASE=${BASE:-http://localhost:8080}

echo "== 1. Health check =="
curl -s "$BASE/health"; echo

echo "== 2. Доступ к задачам без токена (ожидаем 401) =="
curl -s -o /dev/null -w "%{http_code}\n" "$BASE/api/v1/tasks?team_id=1"

echo "== 3. Регистрация пользователя =="
curl -s -X POST "$BASE/api/v1/register" \
  -H 'Content-Type: application/json' \
  -d '{"email":"test@example.com","password":"secret123","name":"Test User"}'; echo

echo "== 4. Логин (получаем JWT) =="
TOKEN=$(curl -s -X POST "$BASE/api/v1/login" \
  -H 'Content-Type: application/json' \
  -d '{"email":"test@example.com","password":"secret123"}' | grep -o '"token":"[^"]*' | cut -d'"' -f4)
echo "TOKEN=$TOKEN"

echo "== 5. Создание команды =="
curl -s -X POST "$BASE/api/v1/teams" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"My Team"}'; echo

echo "== 6. Список команд =="
curl -s "$BASE/api/v1/teams" -H "Authorization: Bearer $TOKEN"; echo

echo "== 7. Создание задачи (team_id=1) =="
curl -s -X POST "$BASE/api/v1/tasks" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"team_id":1,"title":"Первая задача","description":"Описание","status":"todo"}'; echo

echo "== 8. Список задач (фильтр по команде, team_id обязателен) =="
curl -s "$BASE/api/v1/tasks?team_id=1" -H "Authorization: Bearer $TOKEN"; echo

echo "== 9. Задачи чужой команды (ожидаем 403/404) =="
curl -s -o /dev/null -w "%{http_code}\n" "$BASE/api/v1/tasks?team_id=999" -H "Authorization: Bearer $TOKEN"

echo "== 10. Обновление задачи + история =="
curl -s -X PUT "$BASE/api/v1/tasks/1" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"title":"Обновлённая задача","status":"in_progress"}'; echo
curl -s "$BASE/api/v1/tasks/1/history" -H "Authorization: Bearer $TOKEN"; echo

echo "== 11. Комментарии =="
curl -s -X POST "$BASE/api/v1/tasks/1/comments" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"content":"Комментарий к задаче"}'; echo
curl -s "$BASE/api/v1/tasks/1/comments" -H "Authorization: Bearer $TOKEN"; echo

echo "== 12. Статистика команды (только owner/admin) =="
curl -s "$BASE/api/v1/teams/1/stats" -H "Authorization: Bearer $TOKEN"; echo
