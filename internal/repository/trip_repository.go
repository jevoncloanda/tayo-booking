package repository

import (
	"context"
	"tayo-booking/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type TripRepository struct {
	DB *pgx.Conn
}

func NewTripRepository(db *pgx.Conn) *TripRepository {
	return &TripRepository{DB: db}
}

func (r *TripRepository) GetTripByID(ctx context.Context, tripID uuid.UUID) (*models.Trip, error) {
	query := `SELECT id, route_id, bus_id, departure_time, arrival_time, price, created_at
              FROM trips WHERE id = $1`

	row := r.DB.QueryRow(ctx, query, tripID)
	var trip models.Trip
	err := row.Scan(
		&trip.ID,
		&trip.RouteID,
		&trip.BusID,
		&trip.DepartureTime,
		&trip.ArrivalTime,
		&trip.Price,
		&trip.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &trip, nil
}

func (r *TripRepository) GetAvailableSeats(ctx context.Context, tripID uuid.UUID) ([]models.Seat, error) {
	query := `
        SELECT s.id, s.bus_id, s.seat_number, s.created_at
        FROM seats s
        JOIN trips t ON t.bus_id = s.bus_id
        WHERE t.id = $1
        AND s.id NOT IN (
            SELECT seat_id FROM bookings
            WHERE trip_id = $1
            AND status = 'confirmed'
            AND seat_id IS NOT NULL
        )
        ORDER BY s.seat_number
    `

	rows, err := r.DB.Query(ctx, query, tripID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var seats []models.Seat
	for rows.Next() {
		var seat models.Seat
		err := rows.Scan(&seat.ID, &seat.BusID, &seat.SeatNumber, &seat.CreatedAt)
		if err != nil {
			return nil, err
		}
		seats = append(seats, seat)
	}
	return seats, nil
}
