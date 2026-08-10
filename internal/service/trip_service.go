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

type TripService struct {
	TripRepo *repository.TripRepository
}

func NewTripService(tripRepo *repository.TripRepository) *TripService {
	return &TripService{TripRepo: tripRepo}
}

func (s *TripService) SearchTrips(ctx context.Context, fromStopID, toStopID *uuid.UUID, date *string) ([]models.TripSummary, error) {
	filter := repository.TripFilter{
		FromStopID: fromStopID,
		ToStopID:   toStopID,
		Date:       date,
	}
	trips, err := s.TripRepo.SearchTrips(ctx, filter)
	if err != nil {
		return nil, errors.New("failed to search trips")
	}
	if trips == nil {
		trips = []models.TripSummary{}
	}
	return trips, nil
}

func (s *TripService) GetTripDetail(ctx context.Context, tripID uuid.UUID) (*models.TripDetail, error) {
	detail, err := s.TripRepo.GetTripDetail(ctx, tripID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("trip not found")
		}
		return nil, errors.New("failed to get trip")
	}
	return detail, nil
}

func (s *TripService) CreateTrip(ctx context.Context, routeID, busID uuid.UUID, departureTime, arrivalTime time.Time, price float64) (*models.Trip, error) {
	trip := &models.Trip{
		ID:            uuid.New(),
		RouteID:       routeID,
		BusID:         busID,
		DepartureTime: departureTime,
		ArrivalTime:   arrivalTime,
		Price:         price,
		CreatedAt:     time.Now(),
	}
	if err := s.TripRepo.CreateTrip(ctx, trip); err != nil {
		return nil, errors.New("failed to create trip")
	}
	return trip, nil
}

func (s *TripService) GetSeatsForSegment(ctx context.Context, tripID, fromStopID, toStopID uuid.UUID) ([]repository.SeatAvailability, error) {
	// Validate trip exists first
	_, err := s.TripRepo.GetTripDetail(ctx, tripID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("trip not found")
		}
		return nil, errors.New("failed to get trip")
	}

	seats, err := s.TripRepo.GetSeatsForSegment(ctx, tripID, fromStopID, toStopID)
	if err != nil {
		return nil, errors.New("failed to get seat availability")
	}
	return seats, nil
}
