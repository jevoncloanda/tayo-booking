package routes

import (
	"github.com/gin-gonic/gin"
	"tayo-booking/internal/handlers"
)

func SetupRouter() *gin.Engine {
	r := gin.Default()

	r.GET("/hello", handlers.HelloHandler)

	return r
}