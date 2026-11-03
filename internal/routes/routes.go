package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"testovoe/internal/auth"
	"testovoe/internal/handlers"
	"testovoe/internal/repository"
	"testovoe/internal/service"
	"testovoe/pkg/middleware"

	"database/sql"
)

func SetupRoutes(db *sql.DB, jwtService *auth.JWTService) *chi.Mux {
	r := chi.NewRouter()

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		render.JSON(w, r, map[string]string{"status": "ok"})
	})

	// Репозитории и сервисы
	userRepo := repository.NewUserRepo(db)
	teamRepo := repository.NewTeamRepo(db)
	authService := service.NewAuthService(userRepo, jwtService)
	teamService := service.NewTeamService(teamRepo, userRepo)

	// Public routes
	authHandler := handlers.NewAuthHandler(authService)
	r.Post("/api/v1/register", authHandler.Register)
	r.Post("/api/v1/login", authHandler.Login)

	// Protected routes
	authMiddleware := middleware.NewAuthMiddleware(jwtService)
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware.RequireAuth)

		// Team routes
		teamHandler := handlers.NewTeamHandler(teamService)
		r.Post("/api/v1/teams", teamHandler.CreateTeam)
		r.Get("/api/v1/teams", teamHandler.GetTeams)
		r.Get("/api/v1/teams/{id}/members", teamHandler.GetMembers)
		r.Post("/api/v1/teams/{id}/invite", teamHandler.InviteUser)

		// Task routes (TODO: реализовать в рамках раздела "API: задачи и отчет")
		taskHandler := handlers.NewTaskHandler()
		r.Post("/api/v1/tasks", taskHandler.CreateTask)
		r.Get("/api/v1/tasks", taskHandler.GetTasks)
		r.Put("/api/v1/tasks/{id}", taskHandler.UpdateTask)
		r.Get("/api/v1/tasks/{id}/history", taskHandler.GetTaskHistory)
		r.Post("/api/v1/tasks/{id}/comments", taskHandler.CreateComment)
		r.Get("/api/v1/tasks/{id}/comments", taskHandler.GetComments)
		r.Get("/api/v1/teams/{team_id}/stats", taskHandler.GetTeamStats)
	})

	return r
}
