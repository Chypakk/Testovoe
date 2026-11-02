# Task Management API

Проект для управления задачами внутри команд с ролевой моделью, историей изменений, кешированием списка задач и SQL отчетом.

## Стек технологий

- Go
- Chi (HTTP роутер)
- MySQL
- Redis
- JWT
- Swagger/OpenAPI

## Структура проекта

```
├── cmd/server/          # Основной файл приложения
├── internal/           # Внутренние пакеты
│   ├── handlers/       # HTTP обработчики
│   ├── services/       # Бизнес логика
│   ├── models/         # Модели данных
│   ├── repositories/   # Слой доступа к данным
│   ├── database/       # Работа с БД и Redis
│   └── auth/          # Аутентификация и JWT
├── pkg/              # Общие пакеты
│   ├── utils/        # Утилиты
│   └── middleware/   # Middleware
├── configs/          # Конфигурация
├── docs/            # Документация
└── migrations/      # Миграции БД
```

## Запуск проекта

### Требования

- Go 1.21+
- MySQL 8.0+
- Redis 6.0+

### Настройка

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
# TODO: Добавить команды для миграций
```

5. Запустите приложение:
```bash
go run cmd/server/main.go
```

Приложение будет доступно на `http://localhost:8080`

## API Эндпоинты

### Аутентификация
- `POST /api/v1/register` - Регистрация пользователя
- `POST /api/v1/login` - Аутентификация пользователя

### Команды
- `POST /api/v1/teams` - Создание команды
- `GET /api/v1/teams` - Получение списка команд
- `POST /api/v1/teams/{id}/invite` - Приглашение пользователя в команду

### Задачи
- `POST /api/v1/tasks` - Создание задачи
- `GET /api/v1/tasks` - Получение списка задач с фильтрацией
- `PUT /api/v1/tasks/{id}` - Обновление задачи
- `GET /api/v1/tasks/{id}/history` - История изменений задачи
- `POST /api/v1/tasks/{id}/comments` - Добавление комментария
- `GET /api/v1/tasks/{id}/comments` - Получение комментариев задачи
- `GET /api/v1/teams/{team_id}/stats` - Статистика по команде

### Документация

Swagger/OpenAPI документация будет доступна по адресу: `http://localhost:8080/docs`

## Ролевая модель

- **Owner (Создатель)**: Полный доступ в рамках своей команды
- **Admin (Администратор)**: Может приглашать участников, редактировать задачи, смотреть аналитику
- **Member (Участник)**: Может создавать задачи, редактировать свои задачи, видеть комментарии

## Тестирование

```bash
# Запуск всех тестов
go test ./...

# Запуск тестов с покрытием
go test -cover ./...
```

## Docker (TODO)

В будущем будет добавлена поддержка Docker и Docker Compose для легкого развертывания.

## Лицензия

MIT