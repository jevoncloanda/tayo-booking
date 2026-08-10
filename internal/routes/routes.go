package routes

import (
	"tayo-booking/internal/handlers"

	"github.com/gin-gonic/gin"
)

func SetupRouter(
	helloHandler *handlers.HelloHandler,
	authHandler *handlers.AuthHandler,
	tripHandler *handlers.TripHandler,
) *gin.Engine {
	r := gin.Default()

	r.GET("/hello", helloHandler.Handle)

	// Auth
	r.POST("/auth/register", authHandler.Register)
	r.POST("/auth/login", authHandler.Login)
	r.POST("/auth/refresh", authHandler.Refresh)
	r.POST("/auth/logout", authHandler.Logout)
	r.GET("/me", handlers.AuthMiddleware(), authHandler.Me)

	// Public trip routes
	r.GET("/trips", tripHandler.Search)
	r.GET("/trips/:id", tripHandler.GetByID)
	r.GET("/trips/:id/seats", tripHandler.GetSeats)

	// Admin routes
	admin := r.Group("/admin")
	admin.Use(handlers.AuthMiddleware(), handlers.AdminMiddleware())
	{
		admin.POST("/trips", tripHandler.Create)
	}

	return r
}
