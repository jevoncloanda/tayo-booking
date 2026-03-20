package handlers

import "github.com/gin-gonic/gin"

type HelloHandler struct{}

func (h *HelloHandler) Handle(c *gin.Context) {
	c.JSON(200, gin.H{
		"message": "Hello from TAYO API",
	})
}
