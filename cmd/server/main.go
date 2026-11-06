// @title Task Manager API
// @version 1.0
// @description REST API для управления задачами внутри команд: роли, история изменений, комментарии, кеш и SQL-отчёт.
// @BasePath /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description JWT Bearer token. Формат: Bearer {token}
package main

import (
	"net/http"

	"testovoe/internal/auth"
	"testovoe/internal/configs"
	"testovoe/internal/database"
	"testovoe/internal/logger"
	"testovoe/internal/routes"
)

func main() {
	// Инициализируем логгер
	logger.Init()

	// Загружаем .env (если не найден — используем переменные окружения/значения по умолчанию)
	if err := configs.LoadDotEnv(".env"); err != nil {
		logger.Error("Failed to load .env file", err)
	}

	cfg, err := configs.LoadConfig()
	if err != nil {
		logger.Error("Failed to load config", err)
	}

	db, err := database.NewDatabase(cfg.Database)
	if err != nil {
		logger.Error("Failed to connect to database", err)
	}
	defer db.Close()

	if err := db.RunMigrations(); err != nil {
		logger.Error("Failed to run migrations", err)
	}
	logger.Info("Migrations applied successfully")

	jwtService := auth.NewJWTService(cfg.JWT.Secret)
	r := routes.SetupRoutes(db.GetDB(), jwtService, cfg) // Передаем cfg

	addr := ":" + cfg.Server.Port
	logger.Info("Server starting on "+addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		logger.Error("Server failed", err)
	}
}