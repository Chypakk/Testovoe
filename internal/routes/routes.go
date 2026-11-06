package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"testovoe/internal/auth"
	"testovoe/internal/cache"
	"testovoe/internal/configs"
	"testovoe/internal/handlers"
	"testovoe/internal/logger"
	"testovoe/internal/repository"
	"testovoe/internal/service"
	"testovoe/pkg/middleware"

	"database/sql"
)

func SetupRoutes(db *sql.DB, jwtService *auth.JWTService, cfg *configs.Config) *chi.Mux {
	logger.Info("Setting up routes")

	r := chi.NewRouter()

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("Health check requested",
			"method", r.Method,
			"path", r.URL.Path,
			"ip", r.RemoteAddr,
		)
		render.JSON(w, r, map[string]string{"status": "ok"})
	})

	// Swagger documentation
	swaggerHandler := handlers.NewSwaggerHandler()
	swaggerHandler.RegisterSwaggerRoutes(r)

	// Репозитории и сервисы
	logger.Info("Initializing repositories and services")
	taskCache := cache.NewTaskCache(cfg.Redis) // Используем фабрику
	userRepo := repository.NewUserRepo(db, cfg)
	teamRepo := repository.NewTeamRepo(db, cfg)
	taskRepo := repository.NewTaskRepo(db, cfg)

	authService := service.NewAuthService(userRepo, jwtService)
	teamService := service.NewTeamService(teamRepo, userRepo)
	taskService := service.NewTaskService(taskRepo, teamRepo, userRepo, taskCache)

	// Public routes
	logger.Info("Setting up public routes")
	authHandler := handlers.NewAuthHandler(authService)
	r.Post("/api/v1/register", authHandler.Register)
	r.Post("/api/v1/login", authHandler.Login)

	// Protected routes
	logger.Info("Setting up protected routes")
	authMiddleware := middleware.NewAuthMiddleware(jwtService)
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware.RequireAuth)

		// Team routes
		teamHandler := handlers.NewTeamHandler(teamService)
		r.Post("/api/v1/teams", teamHandler.CreateTeam)
		r.Get("/api/v1/teams", teamHandler.GetTeams)
		r.Get("/api/v1/teams/{id}/members", teamHandler.GetMembers)
		r.Post("/api/v1/teams/{id}/invite", teamHandler.InviteUser)

		// Task routes
		taskHandler := handlers.NewTaskHandler(taskService)
		r.Post("/api/v1/tasks", taskHandler.CreateTask)
		r.Get("/api/v1/tasks", taskHandler.GetTasks)
		r.Put("/api/v1/tasks/{id}", taskHandler.UpdateTask)
		r.Get("/api/v1/tasks/{id}/history", taskHandler.GetTaskHistory)
		r.Post("/api/v1/tasks/{id}/comments", taskHandler.CreateComment)
		r.Get("/api/v1/tasks/{id}/comments", taskHandler.GetComments)
		r.Get("/api/v1/teams/{team_id}/stats", taskHandler.GetTeamStats)
	})

	logger.Info("Routes setup completed")
	return r
}