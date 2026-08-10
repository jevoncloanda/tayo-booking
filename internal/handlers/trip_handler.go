package handlers

import (
	"net/http"
	"tayo-booking/internal/service"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type TripHandler struct {
	Service *service.TripService
}

func NewTripHandler(svc *service.TripService) *TripHandler {
	return &TripHandler{Service: svc}
}

func (h *TripHandler) Search(c *gin.Context) {
	var fromStopID, toStopID *uuid.UUID
	var date *string

	if raw := c.Query("from_stop_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid from_stop_id"})
			return
		}
		fromStopID = &id
	}

	if raw := c.Query("to_stop_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid to_stop_id"})
			return
		}
		toStopID = &id
	}

	if raw := c.Query("date"); raw != "" {
		date = &raw
	}

	ctx := c.Request.Context()
	trips, err := h.Service.SearchTrips(ctx, fromStopID, toStopID, date)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, trips)
}

func (h *TripHandler) GetByID(c *gin.Context) {
	tripID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid trip ID"})
		return
	}

	ctx := c.Request.Context()
	detail, err := h.Service.GetTripDetail(ctx, tripID)
	if err != nil {
		if err.Error() == "trip not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Trip not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, detail)
}

type CreateTripRequest struct {
	RouteID       string  `json:"route_id"       binding:"required,uuid"`
	BusID         string  `json:"bus_id"         binding:"required,uuid"`
	DepartureTime string  `json:"departure_time" binding:"required"`
	ArrivalTime   string  `json:"arrival_time"   binding:"required"`
	Price         float64 `json:"price"          binding:"required,gt=0"`
}

func (h *TripHandler) Create(c *gin.Context) {
	var req CreateTripRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed", "details": err.Error()})
		return
	}

	routeID, _ := uuid.Parse(req.RouteID)
	busID, _ := uuid.Parse(req.BusID)

	const layout = time.RFC3339
	departureTime, err := time.Parse(layout, req.DepartureTime)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid departure_time format, use RFC3339 (e.g. 2026-07-20T08:00:00Z)"})
		return
	}
	arrivalTime, err := time.Parse(layout, req.ArrivalTime)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid arrival_time format, use RFC3339 (e.g. 2026-07-20T12:00:00Z)"})
		return
	}

	if !arrivalTime.After(departureTime) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "arrival_time must be after departure_time"})
		return
	}

	ctx := c.Request.Context()
	trip, err := h.Service.CreateTrip(ctx, routeID, busID, departureTime, arrivalTime, req.Price)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, trip)
}

func (h *TripHandler) GetSeats(c *gin.Context) {
	tripID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid trip ID"})
		return
	}

	fromStopID, err := uuid.Parse(c.Query("from_stop_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or missing from_stop_id"})
		return
	}

	toStopID, err := uuid.Parse(c.Query("to_stop_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or missing to_stop_id"})
		return
	}

	ctx := c.Request.Context()
	seats, err := h.Service.GetSeatsForSegment(ctx, tripID, fromStopID, toStopID)
	if err != nil {
		if err.Error() == "trip not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Trip not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"seats": seats})
}
