package repository

import (
	"context"
	"fmt"
	"log"
	"strings"
	"tayo-booking/internal/models"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TripRepository struct {
	DB *pgxpool.Pool
}

func NewTripRepository(db *pgxpool.Pool) *TripRepository {
	return &TripRepository{DB: db}
}

type TripFilter struct {
	FromStopID *uuid.UUID
	ToStopID   *uuid.UUID
	Date       *string // "YYYY-MM-DD"
}

// SearchTrips returns all trips matching the optional filter.
// When from/to stop IDs are provided, it also validates that from_stop
// appears before to_stop in route order (correct direction check).
func (r *TripRepository) SearchTrips(ctx context.Context, f TripFilter) ([]models.TripSummary, error) {
	args := []any{}
	argIdx := 1

	query := `
        SELECT
            t.id,
            r.id   AS route_id,
            r.name AS route_name,
            b.id   AS bus_id,
            b.name AS bus_name,
            b.total_seats,
            b.columns_left,
            b.columns_right,
            t.departure_time,
            t.arrival_time,
            t.price,
            fs.id   AS from_stop_id,
            fs.name AS from_stop_name,
            fs.city AS from_stop_city,
            ts.id   AS to_stop_id,
            ts.name AS to_stop_name,
            ts.city AS to_stop_city
        FROM trips t
        JOIN routes r ON r.id = t.route_id
        JOIN buses  b ON b.id = t.bus_id
    `

	where := []string{}

	if f.FromStopID != nil {
		query += fmt.Sprintf(`
        JOIN route_stops frs ON frs.route_id = t.route_id AND frs.stop_id = $%d
        JOIN stops fs ON fs.id = frs.stop_id
        `, argIdx)
		args = append(args, *f.FromStopID)
		argIdx++
	} else {
		query += `
        LEFT JOIN route_stops frs ON false
        LEFT JOIN stops fs ON false
        `
	}

	if f.ToStopID != nil {
		query += fmt.Sprintf(`
        JOIN route_stops trs ON trs.route_id = t.route_id AND trs.stop_id = $%d
        JOIN stops ts ON ts.id = trs.stop_id
        `, argIdx)
		args = append(args, *f.ToStopID)
		argIdx++
		// Ensure direction: from comes before to
		if f.FromStopID != nil {
			where = append(where, `frs.stop_order < trs.stop_order`)
		}
	} else {
		query += `
        LEFT JOIN route_stops trs ON false
        LEFT JOIN stops ts ON false
        `
	}

	if f.Date != nil {
		where = append(where, fmt.Sprintf(`t.departure_time::date = $%d::date`, argIdx))
		args = append(args, *f.Date)
		argIdx++
	}

	if len(where) > 0 {
		query += " WHERE "
		for i, w := range where {
			if i > 0 {
				query += " AND "
			}
			query += w
		}
	}

	query += ` ORDER BY t.departure_time ASC`

	rows, err := r.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trips []models.TripSummary
	for rows.Next() {
		var trip models.TripSummary
		var fromID, toID *uuid.UUID
		var fromName, fromCity, toName, toCity *string

		err := rows.Scan(
			&trip.ID,
			&trip.Route.ID,
			&trip.Route.Name,
			&trip.Bus.ID,
			&trip.Bus.Name,
			&trip.Bus.TotalSeats,
			&trip.Bus.ColumnsLeft,
			&trip.Bus.ColumnsRight,
			&trip.DepartureTime,
			&trip.ArrivalTime,
			&trip.Price,
			&fromID, &fromName, &fromCity,
			&toID, &toName, &toCity,
		)
		if err != nil {
			return nil, err
		}

		if fromID != nil {
			trip.FromStop = &models.StopInfo{ID: *fromID, Name: *fromName, City: fromCity}
		}
		if toID != nil {
			trip.ToStop = &models.StopInfo{ID: *toID, Name: *toName, City: toCity}
		}

		trips = append(trips, trip)
	}
	return trips, nil
}

// GetTripDetail returns a single trip with its full ordered stop list.
func (r *TripRepository) GetTripDetail(ctx context.Context, tripID uuid.UUID) (*models.TripDetail, error) {
	// Fetch trip + route + bus
	tripRow := r.DB.QueryRow(ctx, `
        SELECT
            t.id, r.id, r.name, b.id, b.name, b.total_seats, b.columns_left, b.columns_right,
            t.departure_time, t.arrival_time, t.price
        FROM trips t
        JOIN routes r ON r.id = t.route_id
        JOIN buses  b ON b.id = t.bus_id
        WHERE t.id = $1
    `, tripID)

	var d models.TripDetail
	err := tripRow.Scan(
		&d.ID,
		&d.Route.ID, &d.Route.Name,
		&d.Bus.ID, &d.Bus.Name, &d.Bus.TotalSeats, &d.Bus.ColumnsLeft, &d.Bus.ColumnsRight,
		&d.DepartureTime, &d.ArrivalTime, &d.Price,
	)
	if err != nil {
		return nil, err
	}

	// Fetch ordered stops for the route
	rows, err := r.DB.Query(ctx, `
        SELECT rs.stop_order, s.id, s.name, s.city
        FROM route_stops rs
        JOIN stops s ON s.id = rs.stop_id
        WHERE rs.route_id = $1
        ORDER BY rs.stop_order ASC
    `, d.Route.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var entry models.StopEntry
		err := rows.Scan(&entry.StopOrder, &entry.Stop.ID, &entry.Stop.Name, &entry.Stop.City)
		if err != nil {
			return nil, err
		}
		d.Stops = append(d.Stops, entry)
	}

	return &d, nil
}

func (r *TripRepository) CreateTrip(ctx context.Context, trip *models.Trip) error {
	query := `
        insert into trips (id, route_id, bus_id, departure_time, arrival_time, price, max_cancellation_minutes, created_at)
        values ($1, $2, $3, $4, $5, $6, $7, $8)
    `
	_, err := r.DB.Exec(ctx, query,
		trip.ID,
		trip.RouteID,
		trip.BusID,
		trip.DepartureTime,
		trip.ArrivalTime,
		trip.Price,
		trip.MaxCancellationMinutes,
		trip.CreatedAt,
	)
	if err != nil {
		log.Println("[CreateTrip] error:", err)
	}
	return err
}

// UpdateTripParams holds optional fields for a partial trip update.
type UpdateTripParams struct {
	DepartureTime          *time.Time
	ArrivalTime            *time.Time
	Price                  *float64
	MaxCancellationMinutes *int
}

// UpdateTrip performs a partial update on a trip, only touching provided fields.
// Returns pgx.ErrNoRows if the trip does not exist.
func (r *TripRepository) UpdateTrip(ctx context.Context, tripID uuid.UUID, p UpdateTripParams) (*models.Trip, error) {
	setClauses := []string{}
	args := []any{}
	argIdx := 1

	if p.DepartureTime != nil {
		setClauses = append(setClauses, fmt.Sprintf("departure_time = $%d", argIdx))
		args = append(args, *p.DepartureTime)
		argIdx++
	}
	if p.ArrivalTime != nil {
		setClauses = append(setClauses, fmt.Sprintf("arrival_time = $%d", argIdx))
		args = append(args, *p.ArrivalTime)
		argIdx++
	}
	if p.Price != nil {
		setClauses = append(setClauses, fmt.Sprintf("price = $%d", argIdx))
		args = append(args, *p.Price)
		argIdx++
	}
	if p.MaxCancellationMinutes != nil {
		setClauses = append(setClauses, fmt.Sprintf("max_cancellation_minutes = $%d", argIdx))
		args = append(args, *p.MaxCancellationMinutes)
		argIdx++
	}

	if len(setClauses) == 0 {
		// nothing to update — fetch and return current state
		return r.getTripByID(ctx, tripID)
	}

	args = append(args, tripID)
	query := fmt.Sprintf(
		"update trips set %s where id = $%d returning id, route_id, bus_id, departure_time, arrival_time, price, max_cancellation_minutes, created_at",
		strings.Join(setClauses, ", "),
		argIdx,
	)

	row := r.DB.QueryRow(ctx, query, args...)
	var trip models.Trip
	err := row.Scan(
		&trip.ID, &trip.RouteID, &trip.BusID,
		&trip.DepartureTime, &trip.ArrivalTime,
		&trip.Price, &trip.MaxCancellationMinutes, &trip.CreatedAt,
	)
	if err != nil {
		log.Println("[UpdateTrip] error:", err)
		return nil, err
	}
	return &trip, nil
}

func (r *TripRepository) getTripByID(ctx context.Context, tripID uuid.UUID) (*models.Trip, error) {
	query := `
		select id, route_id, bus_id, departure_time, arrival_time, price, max_cancellation_minutes, created_at
		from trips where id = $1
	`
	var trip models.Trip
	err := r.DB.QueryRow(ctx, query, tripID).Scan(
		&trip.ID, &trip.RouteID, &trip.BusID,
		&trip.DepartureTime, &trip.ArrivalTime,
		&trip.Price, &trip.MaxCancellationMinutes, &trip.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &trip, nil
}

type SeatAvailability struct {
	ID         uuid.UUID `json:"id"`
	SeatNumber string    `json:"seat_number"`
	SeatRow    *string   `json:"seat_row"`
	SeatColumn *int      `json:"seat_column"`
	Status     string    `json:"status"` // "available" or "booked"
}

func (r *TripRepository) GetSeatsForSegment(ctx context.Context, tripID, fromStopID, toStopID uuid.UUID) ([]SeatAvailability, error) {
	query := `
        WITH trip_route AS (
            SELECT t.bus_id, t.route_id
            FROM trips t
            WHERE t.id = $1
        ),
        stop_orders AS (
            SELECT
                rs.stop_order AS from_order
            FROM route_stops rs
            JOIN trip_route tr ON rs.route_id = tr.route_id
            WHERE rs.stop_id = $2

            UNION ALL

            SELECT
                rs.stop_order AS to_order
            FROM route_stops rs
            JOIN trip_route tr ON rs.route_id = tr.route_id
            WHERE rs.stop_id = $3
        ),
        from_order AS (
            SELECT stop_order FROM route_stops rs
            JOIN trip_route tr ON rs.route_id = tr.route_id
            WHERE rs.stop_id = $2
        ),
        to_order AS (
            SELECT stop_order FROM route_stops rs
            JOIN trip_route tr ON rs.route_id = tr.route_id
            WHERE rs.stop_id = $3
        ),
        booked_seats AS (
            SELECT b.seat_id
            FROM bookings b
            JOIN route_stops bfrs ON bfrs.stop_id = b.from_stop_id
                AND bfrs.route_id = (SELECT route_id FROM trip_route)
            JOIN route_stops btrs ON btrs.stop_id = b.to_stop_id
                AND btrs.route_id = (SELECT route_id FROM trip_route)
            WHERE b.trip_id = $1
              AND b.status IN ('confirmed', 'pending')
              AND bfrs.stop_order < (SELECT stop_order FROM to_order)
              AND btrs.stop_order > (SELECT stop_order FROM from_order)
        )
        SELECT
            s.id,
            s.seat_number,
            s.seat_row,
            s.seat_column,
            CASE WHEN bs.seat_id IS NOT NULL THEN 'booked' ELSE 'available' END AS status
        FROM seats s
        JOIN trip_route tr ON s.bus_id = tr.bus_id
        LEFT JOIN booked_seats bs ON bs.seat_id = s.id
        ORDER BY s.seat_row, s.seat_column
    `

	rows, err := r.DB.Query(ctx, query, tripID, fromStopID, toStopID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var seats []SeatAvailability
	for rows.Next() {
		var s SeatAvailability
		err := rows.Scan(&s.ID, &s.SeatNumber, &s.SeatRow, &s.SeatColumn, &s.Status)
		if err != nil {
			return nil, err
		}
		seats = append(seats, s)
	}

	if seats == nil {
		seats = []SeatAvailability{}
	}

	return seats, nil
}
