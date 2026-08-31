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

type StopService struct {
	StopRepo *repository.StopRepository
}

func NewStopService(stopRepo *repository.StopRepository) *StopService {
	return &StopService{StopRepo: stopRepo}
}

func (s *StopService) CreateStop(ctx context.Context, name, city string) (*models.Stop, error) {
	existing, err := s.StopRepo.GetStopByNameAndCity(ctx, name, city)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("failed to create stop")
	}
	if existing != nil {
		return nil, errors.New("stop already exists")
	}

	stop := &models.Stop{
		ID:        uuid.New(),
		Name:      name,
		City:      &city,
		CreatedAt: time.Now(),
	}
	if err := s.StopRepo.CreateStop(ctx, stop); err != nil {
		return nil, errors.New("failed to create stop")
	}
	return stop, nil
}

func (s *StopService) GetAllStops(ctx context.Context) ([]models.Stop, error) {
	stops, err := s.StopRepo.GetAllStops(ctx)
	if err != nil {
		return nil, errors.New("failed to fetch stops")
	}
	if stops == nil {
		stops = []models.Stop{}
	}
	return stops, nil
}
