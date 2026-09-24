package repository

import (
	"context"
	"log"
	"tayo-booking/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RouteRepository struct {
	DB *pgxpool.Pool
}

func NewRouteRepository(db *pgxpool.Pool) *RouteRepository {
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

// ListRoutes returns every route with its ordered stops. Stops are fetched in a
// single second query and grouped in memory rather than per-route (N+1).
func (r *RouteRepository) ListRoutes(ctx context.Context) ([]models.RouteDetail, error) {
	rows, err := r.DB.Query(ctx, `
		select id, name, created_at
		from routes
		order by created_at desc
	`)
	if err != nil {
		log.Println("[ListRoutes] query routes error:", err)
		return nil, err
	}
	defer rows.Close()

	routes := []models.RouteDetail{}
	index := map[uuid.UUID]int{}
	for rows.Next() {
		var d models.RouteDetail
		if err := rows.Scan(&d.ID, &d.Name, &d.CreatedAt); err != nil {
			log.Println("[ListRoutes] scan route error:", err)
			return nil, err
		}
		d.Stops = []models.StopEntry{}
		index[d.ID] = len(routes)
		routes = append(routes, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(routes) == 0 {
		return routes, nil
	}

	stopRows, err := r.DB.Query(ctx, `
		select rs.route_id, rs.stop_order, s.id, s.name, s.city
		from route_stops rs
		join stops s on s.id = rs.stop_id
		order by rs.route_id, rs.stop_order asc
	`)
	if err != nil {
		log.Println("[ListRoutes] query route stops error:", err)
		return nil, err
	}
	defer stopRows.Close()

	for stopRows.Next() {
		var routeID uuid.UUID
		var entry models.StopEntry
		if err := stopRows.Scan(&routeID, &entry.StopOrder, &entry.Stop.ID, &entry.Stop.Name, &entry.Stop.City); err != nil {
			log.Println("[ListRoutes] scan route stop error:", err)
			return nil, err
		}
		if i, ok := index[routeID]; ok {
			routes[i].Stops = append(routes[i].Stops, entry)
		}
	}
	return routes, stopRows.Err()
}
