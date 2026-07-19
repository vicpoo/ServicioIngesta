//src/infrastructure/routes/routes.go
// Package routes define las rutas HTTP del servicio. Ingesta es
// principalmente un consumidor de RabbitMQ; el servidor HTTP en el puerto
// 8001 existe para health checks (y como base para futuros endpoints de
// administración/métricas).
package routes

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/kajve/ingesta-iot/src/infrastructure/controllers"
)

// NewRouter arma el router HTTP.
func NewRouter() *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	healthController := controllers.NewHealthController()
	r.Get("/health", healthController.Health)

	return r
}
