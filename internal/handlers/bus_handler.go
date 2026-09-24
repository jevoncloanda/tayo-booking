package handlers

import (
	"net/http"
	"tayo-booking/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type BusHandler struct {
	Service *service.BusService
}

func NewBusHandler(svc *service.BusService) *BusHandler {
	return &BusHandler{Service: svc}
}

type CreateBusRequest struct {
	Name         string `json:"name"          binding:"required"`
	TotalRows    int    `json:"total_rows"    binding:"required,min=1"`
	ColumnsLeft  int    `json:"columns_left"  binding:"required,min=1"`
	ColumnsRight int    `json:"columns_right" binding:"required,min=1"`
}

func (h *BusHandler) Create(c *gin.Context) {
	var req CreateBusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed", "details": err.Error()})
		return
	}

	ctx := c.Request.Context()
	bus, err := h.Service.CreateBus(ctx, req.Name, req.TotalRows, req.ColumnsLeft, req.ColumnsRight)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, bus)
}

type UpdateSeatRequest struct {
	SeatNumber string `json:"seat_number" binding:"required"`
}

func (h *BusHandler) UpdateSeat(c *gin.Context) {
	seatID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid seat ID"})
		return
	}

	var req UpdateSeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed", "details": err.Error()})
		return
	}

	ctx := c.Request.Context()
	seat, err := h.Service.UpdateSeat(ctx, seatID, req.SeatNumber)
	if err != nil {
		if err.Error() == "duplicate seat number" {
			c.JSON(http.StatusConflict, gin.H{"error": "Seat number already exists on this bus"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, seat)
}

func (h *BusHandler) List(c *gin.Context) {
	buses, err := h.Service.ListBuses(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, buses)
}
