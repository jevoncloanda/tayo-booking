package routes

import (
	"tayo-booking/internal/handlers"

	"github.com/gin-gonic/gin"
)

// Accept handler dependencies
func SetupRouter(helloHandler *handlers.HelloHandler, authHandler *handlers.AuthHandler) *gin.Engine {
	r := gin.Default()

	r.GET("/hello", helloHandler.Handle)

	// Auth routes
	r.POST("/auth/register", authHandler.Register)
	r.POST("/auth/login", authHandler.Login)
	r.POST("/auth/refresh", authHandler.Refresh)
	r.POST("/auth/logout", authHandler.Logout)
	r.GET("/me", handlers.AuthMiddleware(), authHandler.Me)

	return r
}
