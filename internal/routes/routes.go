package routes

import (
	"tayo-booking/internal/handlers"

	"github.com/gin-gonic/gin"
)

// Accept handler dependencies
func SetupRouter(helloHandler *handlers.HelloHandler, userHandler *handlers.UserHandler) *gin.Engine {
	r := gin.Default()

	r.GET("/hello", helloHandler.Handle)
	r.POST("/users/register", userHandler.Register)

	return r
}
