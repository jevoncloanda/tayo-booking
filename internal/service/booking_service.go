package service

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"tayo-booking/internal/domain"
	"tayo-booking/internal/models"
	"tayo-booking/internal/repository"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type BookingService struct {
	BookingRepo *repository.BookingRepository
}

func NewBookingService(bookingRepo *repository.BookingRepository) *BookingService {
	return &BookingService{BookingRepo: bookingRepo}
}

// CreateBooking validates inputs, checks seat availability, and creates the booking atomically.
func (s *BookingService) CreateBooking(
	ctx context.Context,
	userID, tripID, seatID, fromStopID, toStopID uuid.UUID,
	idempotencyKey string,
) (*models.Booking, bool, error) {
	now := time.Now()
	booking := &models.Booking{
		ID:         uuid.New(),
		UserID:     userID,
		TripID:     tripID,
		SeatID:     &seatID,
		FromStopID: &fromStopID,
		ToStopID:   &toStopID,
		Status:     "pending",
		CreatedAt:  now,
	}

	fingerprint := sha256.Sum256([]byte(
		"booking:v1\x00" + tripID.String() + "\x00" + seatID.String() + "\x00" +
			fromStopID.String() + "\x00" + toStopID.String(),
	))
	replayed, err := s.BookingRepo.CreateBooking(ctx, booking, idempotencyKey, fingerprint[:])
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrSeatUnavailable),
			errors.Is(err, domain.ErrFromStopNotOnRoute),
			errors.Is(err, domain.ErrToStopNotOnRoute),
			errors.Is(err, domain.ErrInvalidSegment),
			errors.Is(err, domain.ErrSeatNotOnBus),
			errors.Is(err, domain.ErrIdempotencyConflict):
			return nil, false, err
		}
		return nil, false, fmt.Errorf("failed to create booking: %w", err)
	}

	return booking, replayed, nil
}

// GetUserBookings returns all bookings for an authenticated user.
func (s *BookingService) GetUserBookings(ctx context.Context, userID uuid.UUID) ([]models.BookingDetail, error) {
	bookings, err := s.BookingRepo.GetBookingsByUser(ctx, userID)
	if err != nil {
		return nil, errors.New("failed to get bookings")
	}
	return bookings, nil
}

// GetBookingByID returns a single booking, enforcing ownership.
func (s *BookingService) GetBookingByID(ctx context.Context, id, userID uuid.UUID) (*models.BookingDetail, error) {
	ownerID, _, err := s.BookingRepo.GetBookingOwner(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrBookingNotFound
		}
		return nil, errors.New("failed to get booking")
	}
	if ownerID != userID {
		return nil, domain.ErrForbidden
	}

	detail, err := s.BookingRepo.GetBookingByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrBookingNotFound
		}
		return nil, errors.New("failed to get booking")
	}
	return detail, nil
}

// CancelBooking cancels a booking with ownership, status, and time-window validation.
func (s *BookingService) CancelBooking(ctx context.Context, id, userID uuid.UUID) error {
	ownerID, status, err := s.BookingRepo.GetBookingOwner(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrBookingNotFound
		}
		return errors.New("failed to get booking")
	}

	if ownerID != userID {
		return domain.ErrForbidden
	}

	if status == "cancelled" {
		return domain.ErrBookingAlreadyCancelled
	}

	if status == "confirmed" {
		return domain.ErrConfirmedBooking
	}

	// Check cancellation window via trip.
	trip, err := s.BookingRepo.GetTripForBookingCancellation(ctx, id)
	if err != nil {
		return errors.New("failed to get trip details")
	}
	if trip.MaxCancellationMinutes > 0 {
		cutoff := trip.DepartureTime.Add(-time.Duration(trip.MaxCancellationMinutes) * time.Minute)
		if time.Now().After(cutoff) {
			return domain.ErrCancellationWindowPassed
		}
	}

	if err := s.BookingRepo.CancelBooking(ctx, id); err != nil {
		return errors.New("failed to cancel booking")
	}
	return nil
}

// GetAllBookings returns all bookings for admin with optional filters.
func (s *BookingService) GetAllBookings(ctx context.Context, status *string, tripID *uuid.UUID) ([]models.BookingDetailAdmin, error) {
	filter := repository.BookingFilter{
		Status: status,
		TripID: tripID,
	}
	bookings, err := s.BookingRepo.GetAllBookings(ctx, filter)
	if err != nil {
		return nil, errors.New("failed to get bookings")
	}
	return bookings, nil
}

// UpdateBookingStatus updates a booking's status for admin.
// Only allows confirmed or cancelled as target statuses.
func (s *BookingService) UpdateBookingStatus(ctx context.Context, id uuid.UUID, status string) (*models.BookingDetailAdmin, error) {
	ownerID, currentStatus, err := s.BookingRepo.GetBookingOwner(ctx, id)
	_ = ownerID
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrBookingNotFound
		}
		return nil, errors.New("failed to get booking")
	}

	if currentStatus == status {
		return nil, domain.ErrBookingStatusUnchanged
	}

	if err := s.BookingRepo.UpdateBookingStatus(ctx, id, status); err != nil {
		return nil, errors.New("failed to update booking status")
	}

	detail, err := s.BookingRepo.GetAdminBookingByID(ctx, id)
	if err != nil {
		return nil, errors.New("failed to get updated booking")
	}
	return detail, nil
}
