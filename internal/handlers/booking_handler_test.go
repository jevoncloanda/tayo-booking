package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"tayo-booking/internal/domain"

	"github.com/gin-gonic/gin"
)

func TestWriteBookingError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"invalid segment", domain.ErrInvalidSegment, http.StatusBadRequest},
		{"seat conflict", domain.ErrSeatUnavailable, http.StatusConflict},
		{"idempotency conflict", domain.ErrIdempotencyConflict, http.StatusConflict},
		{"forbidden", domain.ErrForbidden, http.StatusForbidden},
		{"not found", domain.ErrBookingNotFound, http.StatusNotFound},
		{"cancellation policy", domain.ErrCancellationWindowPassed, http.StatusUnprocessableEntity},
		{"wrapped domain error", errors.Join(errors.New("context"), domain.ErrSeatUnavailable), http.StatusConflict},
		{"unexpected", errors.New("database unavailable"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			writeBookingError(context, tt.err)
			if recorder.Code != tt.want {
				t.Fatalf("expected status %d, got %d", tt.want, recorder.Code)
			}
		})
	}
}
