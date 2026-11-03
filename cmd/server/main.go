package main

import (
	"log"
	"net/http"

	"testovoe/internal/configs"
	"testovoe/internal/database"
	"testovoe/internal/routes"
)

func main() {
	// Загружаем .env (если не найден — используем переменные окружения/значения по умолчанию)
	if err := configs.LoadDotEnv(".env"); err != nil {
		log.Printf("Warning: failed to load .env file: %v", err)
	}

	cfg, err := configs.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := database.NewDatabase(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.RunMigrations(); err != nil {
		log.Printf("Warning: failed to run migrations: %v", err)
	} else {
		log.Println("Migrations applied successfully")
	}

	r := routes.SetupRoutes()

	addr := ":" + cfg.Server.Port
	log.Printf("Server starting on %s", addr)

	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}