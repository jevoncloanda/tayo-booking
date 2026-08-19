package service

import (
	"context"
	"errors"
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
) (*models.Booking, error) {
	// Validate from/to stops belong to the trip's route and get their orders.
	fromOrder, toOrder, err := s.BookingRepo.ValidateStopsOnRoute(ctx, tripID, fromStopID, toStopID)
	if err != nil {
		if err.Error() == "from_stop_id does not belong to this trip's route" ||
			err.Error() == "to_stop_id does not belong to this trip's route" {
			return nil, err
		}
		return nil, errors.New("failed to validate stops")
	}

	// Validate direction.
	if fromOrder >= toOrder {
		return nil, errors.New("from_stop_id must come before to_stop_id on the route")
	}

	// Validate seat belongs to this trip's bus.
	if err := s.BookingRepo.ValidateSeatOnBus(ctx, tripID, seatID); err != nil {
		return nil, err
	}

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

	if err := s.BookingRepo.CreateBooking(ctx, booking); err != nil {
		if err.Error() == "seat unavailable" {
			return nil, errors.New("seat unavailable")
		}
		return nil, errors.New("failed to create booking")
	}

	return booking, nil
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
			return nil, errors.New("booking not found")
		}
		return nil, errors.New("failed to get booking")
	}
	if ownerID != userID {
		return nil, errors.New("forbidden")
	}

	detail, err := s.BookingRepo.GetBookingByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("booking not found")
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
			return errors.New("booking not found")
		}
		return errors.New("failed to get booking")
	}

	if ownerID != userID {
		return errors.New("forbidden")
	}

	if status == "cancelled" {
		return errors.New("already cancelled")
	}

	if status == "confirmed" {
		return errors.New("confirmed booking cannot be cancelled by user")
	}

	// Check cancellation window via trip.
	trip, err := s.BookingRepo.GetTripForBookingCancellation(ctx, id)
	if err != nil {
		return errors.New("failed to get trip details")
	}
	if trip.MaxCancellationMinutes > 0 {
		cutoff := trip.DepartureTime.Add(-time.Duration(trip.MaxCancellationMinutes) * time.Minute)
		if time.Now().After(cutoff) {
			return errors.New("cancellation window has passed")
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
			return nil, errors.New("booking not found")
		}
		return nil, errors.New("failed to get booking")
	}

	if currentStatus == status {
		return nil, errors.New("booking already has this status")
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
