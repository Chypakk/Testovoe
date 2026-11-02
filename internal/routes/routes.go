package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"testovoe/internal/handlers"
	"testovoe/pkg/middleware"
)

func SetupRoutes() *chi.Mux {
	r := chi.NewRouter()

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		render.JSON(w, r, map[string]string{"status": "ok"})
	})

	// Public routes
	authHandler := handlers.NewAuthHandler()
	r.Group(func(r chi.Router) {
		r.Post("/api/v1/register", authHandler.Register)
		r.Post("/api/v1/login", authHandler.Login)
	})

	// Protected routes
	authMiddleware := middleware.NewAuthMiddleware()
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware.RequireAuth)

		// Team routes
		teamHandler := handlers.NewTeamHandler()
		r.Post("/api/v1/teams", teamHandler.CreateTeam)
		r.Get("/api/v1/teams", teamHandler.GetTeams)
		r.Post("/api/v1/teams/{id}/invite", teamHandler.InviteUser)

		// Task routes
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