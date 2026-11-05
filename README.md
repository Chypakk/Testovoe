# Task Management API

REST API сервис для управления задачами внутри команд с ролевой моделью, историей изменений, кешированием списка задач и SQL отчетом.

## Сущности и связи

| Сущность | Минимальные поля и связи |
| --- | --- |
| users | id, email, password_hash, name, created_at |
| teams | id, name, created_by, created_at; created_by -> users.id |
| team_members | team_id, user_id, role; уникальная пара team_id - user_id |
| tasks | id, team_id, title, description, status, created_by, assignee_id, created_at, updated_at, closed_at, version |
| task_history | id, task_id, changed_by, changes, created_at |
| task_comments | id, task_id, user_id, content, created_at |

## Стек технологий

- Go
- MySQL
- Redis
- Docker + Docker Compose
- Git
- Swagger/OpenAPI

## Структура проекта

```
├── cmd/               # Команды приложения
│   ├── server/       # Основной файл приложения
│   └── migrate/      # Миграции БД
├── internal/         # Внутренние пакеты
│   ├── handlers/     # HTTP обработчики
│   ├── services/     # Бизнес логика
│   ├── models/       # Модели данных
│   ├── repositories/ # Слой доступа к данным
│   ├── database/     # Работа с БД и Redis
│   └── auth/        # Аутентификация и JWT
├── pkg/             # Общие пакеты
│   ├── utils/       # Утилиты
│   └── middleware/  # Middleware
├── configs/         # Конфигурация
├── docs/           # Документация
├── migrations/     # Миграции БД
├── docker-compose.yml # Docker Compose конфигурация
└── .env.example    # Пример файла окружения
```

## Запуск проекта

### Требования

- Go 1.21+
- MySQL 8.0+
- Redis 6.0+

### Запуск через Docker Compose

1. Клонируйте репозиторий:
```bash
git clone <repository-url>
cd testovoe
```

2. Запустите все сервисы:
```bash
docker-compose up -d
```

Это запустит приложение, MySQL и Redis.

### Запуск через Go

1. Клонируйте репозиторий:
```bash
git clone <repository-url>
cd testovoe
```

2. Установите зависимости:
```bash
go mod tidy
```

3. Настройте переменные окружения (создайте файл `.env`):
```bash
# Сервер
SERVER_PORT=8080

# MySQL
DB_HOST=localhost
DB_PORT=3306
DB_USER=root
DB_PASSWORD=password
DB_NAME=testovoe

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0

# JWT
JWT_SECRET=your-secret-key
```

4. Запустите миграции БД:
```bash
go run cmd/migrate/main.go
```

5. Запустите приложение:
```bash
go run cmd/server/main.go
```

Приложение будет доступно на `http://localhost:8080`

## API Эндпоинты

### Пользователи и команды
- `POST /api/v1/register` - Регистрация пользователя
- `POST /api/v1/login` - Аутентификация пользователя, выдача JWT
- `POST /api/v1/teams` - Создание команды, текущий пользователь становится создателем
- `GET /api/v1/teams` - Список команд текущего пользователя
- `POST /api/v1/teams/{id}/invite` - Добавление пользователя в команду, только создатель или администратор

### Задачи
- `POST /api/v1/tasks` - Создание задачи в команде
- `GET /api/v1/tasks?team_id=1&status=todo&assignee_id=5&limit=20&offset=0` - Список задач команды с фильтрами и пагинацией
- `PUT /api/v1/tasks/{id}` - Обновление задачи с проверкой прав и записью истории
- `GET /api/v1/tasks/{id}/history` - История изменений задачи
- `POST /api/v1/tasks/{id}/comments` - Добавление комментария к задаче
- `GET /api/v1/tasks/{id}/comments` - Список комментариев задачи
- `GET /api/v1/teams/{team_id}/stats` - SQL отчет по конкретной команде

### Документация

Swagger/OpenAPI документация будет доступна по адресу: `http://localhost:8080/docs`

## Ролевая модель

| Роль | Что может делать |
| --- | --- |
| **Создатель (owner)** | Полный доступ в рамках своей команды: приглашать участников, менять роли, назначать администратора или участника, редактировать любые задачи команды, смотреть аналитику команды. |
| **Администратор (admin)** | Может приглашать участников, редактировать любые задачи команды, смотреть аналитику команды. Не может удалить создателя команды или изменить его роль. |
| **Участник (member)** | Может видеть свою команду, задачи команды, историю и комментарии. Может создавать задачи. Может редактировать только задачи, где он создатель или исполнитель, с ограничениями ниже. |

## Правила видимости и доступа

- Команды: пользователь видит только свои команды, где он участник или является создателем.
- Задачи: пользователь не получает задачи чужой команды. GET /tasks требует team_id и проверку участия в команде.
- История и комментарии: доступны только участникам команды этой задачи.
- Аналитика: /teams/{team_id}/stats доступен только создателю или администратору команды и не возвращает данные других команд.
- Исполнитель: должен быть участником той же команды. Назначение пользователя из другой команды запрещено.
- Приглашения и роли: приглашать пользователей может только создатель или администратор. Роль создателя через обычное приглашение не выдается.

## Правила редактирования задач

- Создание задачи: любой участник команды может создать задачу в своей команде. Создателем считается текущий пользователь.
- Редактирование любой задачи: доступно создателю или администратору команды.
- Редактирование своей задачи: создатель задачи может менять название, описание, статус и исполнителя, если новый исполнитель состоит в команде.
- Редактирование назначенной задачи: исполнитель может менять статус и оставлять комментарии, но не должен произвольно переназначать задачу.
- Обычный участник не может редактировать задачу, где он не создатель и не исполнитель.
- История: любое изменение задачи должно записывать старые и новые значения, автора изменения и время изменения.

## SQL отчет

Обязательный отчет: `GET /api/v1/teams/{team_id}/stats`

Отчет возвращает статистику только по одной команде, к которой у пользователя есть доступ создателя или администратора.

**Метрики:**
- Задачи по статусам
- Топ-3 исполнителя по закрытым задачам за 30 дней
- Среднее время закрытия задач
- Количество комментариев по задачам команды

**Особенности:**
- Один осмысленный SQL запрос без N+1
- Используются JOIN, GROUP BY, агрегаты и работа с датами
- Обязательная фильтрация по team_id и проверка прав на команду

## Кеширование

- Список задач команды кешируется в Redis с TTL 5 минут
- Кеш учитывает команду и фильтры
- После изменения задач кеш обновляется или инвалидируется

## Надежность и транзакции

- Транзакции: создание команды и добавление участника выполняются атомарно. Создание или обновление задачи и запись истории выполняются атомарно.
- Конкурентность: конкурентное обновление задачи не приводит к потере изменений. История строится от актуального состояния задачи.
- Конфигурация: все настройки вынесены в переменные окружения, без хардкода в коде приложения.

## Тестирование

```bash
# Запуск всех тестов
go test ./...

# Запуск тестов с покрытием
go test -cover ./...

# Запуск интеграционного теста на SQL отчет
go test -run TestTeamStats ./...
```

## Docker

Проект поддерживает запуск через Docker Compose:

```bash
docker-compose up -d
```

Это запускает приложение, MySQL и Redis.

## Лицензия

MIT

## Примеры запросов

### Регистрация пользователя
```bash
curl -X POST http://localhost:8080/api/v1/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "password123",
    "name": "John Doe"
  }'
```

### Аутентификация
```bash
curl -X POST http://localhost:8080/api/v1/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "password123"
  }'
```

### Создание команды
```bash
curl -X POST http://localhost:8080/api/v1/teams \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{
    "name": "Моя команда"
  }'
```

### Приглашение пользователя в команду
```bash
curl -X POST http://localhost:8080/api/v1/teams/1/invite \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{
    "user_id": 2,
    "role": "member"
  }'
```

### Создание задачи
```bash
curl -X POST http://localhost:8080/api/v1/tasks \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{
    "team_id": 1,
    "title": "Новая задача",
    "description": "Описание задачи",
    "assignee_id": 2
  }'
```

### Получение списка задач с фильтрацией
```bash
curl "http://localhost:8080/api/v1/tasks?team_id=1&status=todo&limit=20&offset=0" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

### Обновление задачи
```bash
curl -X PUT http://localhost:8080/api/v1/tasks/1 \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{
    "title": "Обновленная задача",
    "status": "in_progress"
  }'
```

### Добавление комментария
```bash
curl -X POST http://localhost:8080/api/v1/tasks/1/comments \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{
    "content": "Это комментарий к задаче"
  }'
```

### Получение статистики по команде
```bash
curl "http://localhost:8080/api/v1/teams/1/stats" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```