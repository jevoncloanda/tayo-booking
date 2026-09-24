package handlers

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"tayo-booking/internal/domain"
	"tayo-booking/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type BookingHandler struct {
	Service *service.BookingService
}

func NewBookingHandler(svc *service.BookingService) *BookingHandler {
	return &BookingHandler{Service: svc}
}

type CreateBookingRequest struct {
	TripID     string `json:"trip_id"      binding:"required,uuid"`
	SeatID     string `json:"seat_id"      binding:"required,uuid"`
	FromStopID string `json:"from_stop_id" binding:"required,uuid"`
	ToStopID   string `json:"to_stop_id"   binding:"required,uuid"`
}

func (h *BookingHandler) Create(c *gin.Context) {
	rawUserID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID, err := uuid.Parse(rawUserID.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user ID in token"})
		return
	}

	var req CreateBookingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed", "details": err.Error()})
		return
	}
	idempotencyKey := c.GetHeader("Idempotency-Key")
	if strings.TrimSpace(idempotencyKey) == "" || len(idempotencyKey) > 255 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Idempotency-Key header is required and must be at most 255 characters"})
		return
	}

	tripID, _ := uuid.Parse(req.TripID)
	seatID, _ := uuid.Parse(req.SeatID)
	fromStopID, _ := uuid.Parse(req.FromStopID)
	toStopID, _ := uuid.Parse(req.ToStopID)

	ctx := c.Request.Context()
	booking, replayed, err := h.Service.CreateBooking(ctx, userID, tripID, seatID, fromStopID, toStopID, idempotencyKey)
	if err != nil {
		writeBookingError(c, err)
		return
	}

	if replayed {
		c.Header("Idempotent-Replayed", "true")
	}
	c.JSON(http.StatusCreated, booking)
}

func (h *BookingHandler) List(c *gin.Context) {
	rawUserID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID, err := uuid.Parse(rawUserID.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user ID in token"})
		return
	}

	ctx := c.Request.Context()
	bookings, err := h.Service.GetUserBookings(ctx, userID)
	if err != nil {
		writeBookingError(c, err)
		return
	}

	c.JSON(http.StatusOK, bookings)
}

func (h *BookingHandler) GetByID(c *gin.Context) {
	rawUserID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID, err := uuid.Parse(rawUserID.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user ID in token"})
		return
	}

	bookingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid booking ID"})
		return
	}

	ctx := c.Request.Context()
	booking, err := h.Service.GetBookingByID(ctx, bookingID, userID)
	if err != nil {
		writeBookingError(c, err)
		return
	}

	c.JSON(http.StatusOK, booking)
}

func (h *BookingHandler) Cancel(c *gin.Context) {
	rawUserID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID, err := uuid.Parse(rawUserID.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user ID in token"})
		return
	}

	bookingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid booking ID"})
		return
	}

	ctx := c.Request.Context()
	err = h.Service.CancelBooking(ctx, bookingID, userID)
	if err != nil {
		writeBookingError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Booking cancelled successfully"})
}

// --- Admin handlers ---

func (h *BookingHandler) AdminList(c *gin.Context) {
	var status *string
	if s := c.Query("status"); s != "" {
		status = &s
	}

	var tripID *uuid.UUID
	if raw := c.Query("trip_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid trip_id"})
			return
		}
		tripID = &id
	}

	ctx := c.Request.Context()
	bookings, err := h.Service.GetAllBookings(ctx, status, tripID)
	if err != nil {
		writeBookingError(c, err)
		return
	}

	c.JSON(http.StatusOK, bookings)
}

type UpdateBookingStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

func (h *BookingHandler) AdminUpdateStatus(c *gin.Context) {
	bookingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid booking ID"})
		return
	}

	var req UpdateBookingStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed", "details": err.Error()})
		return
	}

	if req.Status != "confirmed" && req.Status != "cancelled" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Status must be 'confirmed' or 'cancelled'"})
		return
	}

	ctx := c.Request.Context()
	booking, err := h.Service.UpdateBookingStatus(ctx, bookingID, req.Status)
	if err != nil {
		writeBookingError(c, err)
		return
	}

	c.JSON(http.StatusOK, booking)
}

func writeBookingError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrFromStopNotOnRoute),
		errors.Is(err, domain.ErrToStopNotOnRoute),
		errors.Is(err, domain.ErrInvalidSegment),
		errors.Is(err, domain.ErrSeatNotOnBus):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
	case errors.Is(err, domain.ErrBookingNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Booking not found"})
	case errors.Is(err, domain.ErrSeatUnavailable):
		c.JSON(http.StatusConflict, gin.H{"error": "Seat is not available for the requested segment"})
	case errors.Is(err, domain.ErrIdempotencyConflict):
		c.JSON(http.StatusConflict, gin.H{"error": domain.ErrIdempotencyConflict.Error()})
	case errors.Is(err, domain.ErrBookingAlreadyCancelled):
		c.JSON(http.StatusConflict, gin.H{"error": "Booking is already cancelled"})
	case errors.Is(err, domain.ErrBookingStatusUnchanged):
		c.JSON(http.StatusConflict, gin.H{"error": "Booking is already in the requested status"})
	case errors.Is(err, domain.ErrConfirmedBooking):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Confirmed bookings cannot be cancelled by users"})
	case errors.Is(err, domain.ErrCancellationWindowPassed):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Cancellation window has passed"})
	default:
		log.Printf("[booking] unexpected failure: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
	}
}
