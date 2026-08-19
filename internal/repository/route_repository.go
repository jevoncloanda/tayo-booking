package repository

import (
	"context"
	"log"
	"tayo-booking/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type RouteRepository struct {
	DB *pgx.Conn
}

func NewRouteRepository(db *pgx.Conn) *RouteRepository {
	return &RouteRepository{DB: db}
}

func (r *RouteRepository) CreateRoute(ctx context.Context, route *models.Route) error {
	query := `insert into routes (id, name, created_at) values ($1, $2, $3)`
	_, err := r.DB.Exec(ctx, query, route.ID, route.Name, route.CreatedAt)
	if err != nil {
		log.Println("[CreateRoute] error:", err)
	}
	return err
}

func (r *RouteRepository) CreateRouteStop(ctx context.Context, rs *models.RouteStop) error {
	query := `
		insert into route_stops (id, route_id, stop_id, stop_order, created_at)
		values ($1, $2, $3, $4, $5)
	`
	_, err := r.DB.Exec(ctx, query, rs.ID, rs.RouteID, rs.StopID, rs.StopOrder, rs.CreatedAt)
	if err != nil {
		log.Println("[CreateRouteStop] error:", err)
	}
	return err
}

func (r *RouteRepository) GetRouteStopByOrder(ctx context.Context, routeID uuid.UUID, stopOrder int) (*models.RouteStop, error) {
	query := `
		select id, route_id, stop_id, stop_order, created_at
		from route_stops
		where route_id = $1 and stop_order = $2
	`
	row := r.DB.QueryRow(ctx, query, routeID, stopOrder)
	var rs models.RouteStop
	err := row.Scan(&rs.ID, &rs.RouteID, &rs.StopID, &rs.StopOrder, &rs.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &rs, nil
}

func (r *RouteRepository) GetRouteStopByStopID(ctx context.Context, routeID, stopID uuid.UUID) (*models.RouteStop, error) {
	query := `
		select id, route_id, stop_id, stop_order, created_at
		from route_stops
		where route_id = $1 and stop_id = $2
	`
	row := r.DB.QueryRow(ctx, query, routeID, stopID)
	var rs models.RouteStop
	err := row.Scan(&rs.ID, &rs.RouteID, &rs.StopID, &rs.StopOrder, &rs.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &rs, nil
}
