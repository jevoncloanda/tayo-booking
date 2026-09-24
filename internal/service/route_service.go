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

type RouteService struct {
	RouteRepo *repository.RouteRepository
}

func NewRouteService(routeRepo *repository.RouteRepository) *RouteService {
	return &RouteService{RouteRepo: routeRepo}
}

func (s *RouteService) CreateRoute(ctx context.Context, name string) (*models.Route, error) {
	route := &models.Route{
		ID:        uuid.New(),
		Name:      &name,
		CreatedAt: time.Now(),
	}
	if err := s.RouteRepo.CreateRoute(ctx, route); err != nil {
		return nil, errors.New("failed to create route")
	}
	return route, nil
}

func (s *RouteService) CreateRouteStop(ctx context.Context, routeID, stopID uuid.UUID, stopOrder int) (*models.RouteStop, error) {
	// Check duplicate stop_order on this route
	existingOrder, err := s.RouteRepo.GetRouteStopByOrder(ctx, routeID, stopOrder)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("failed to create route stop")
	}
	if existingOrder != nil {
		return nil, errors.New("stop_order already exists on this route")
	}

	// Check duplicate stop on this route
	existingStop, err := s.RouteRepo.GetRouteStopByStopID(ctx, routeID, stopID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("failed to create route stop")
	}
	if existingStop != nil {
		return nil, errors.New("stop already assigned to this route")
	}

	rs := &models.RouteStop{
		ID:        uuid.New(),
		RouteID:   routeID,
		StopID:    stopID,
		StopOrder: stopOrder,
		CreatedAt: time.Now(),
	}
	if err := s.RouteRepo.CreateRouteStop(ctx, rs); err != nil {
		return nil, errors.New("failed to create route stop")
	}
	return rs, nil
}

func (s *RouteService) ListRoutes(ctx context.Context) ([]models.RouteDetail, error) {
	routes, err := s.RouteRepo.ListRoutes(ctx)
	if err != nil {
		return nil, errors.New("failed to list routes")
	}
	return routes, nil
}
