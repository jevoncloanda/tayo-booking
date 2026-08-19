package handlers

import (
	"net/http"
	"tayo-booking/internal/service"

	"github.com/gin-gonic/gin"
)

type StopHandler struct {
	Service *service.StopService
}

func NewStopHandler(svc *service.StopService) *StopHandler {
	return &StopHandler{Service: svc}
}

type CreateStopRequest struct {
	Name string `json:"name" binding:"required"`
	City string `json:"city" binding:"required"`
}

func (h *StopHandler) Create(c *gin.Context) {
	var req CreateStopRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed", "details": err.Error()})
		return
	}

	ctx := c.Request.Context()
	stop, err := h.Service.CreateStop(ctx, req.Name, req.City)
	if err != nil {
		if err.Error() == "duplicate stop" {
			c.JSON(http.StatusConflict, gin.H{"error": "A stop with this name and city already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, stop)
}
