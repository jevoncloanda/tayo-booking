package repository

import (
	"context"
	"log"
	"tayo-booking/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type BusRepository struct {
	DB *pgx.Conn
}

func NewBusRepository(db *pgx.Conn) *BusRepository {
	return &BusRepository{DB: db}
}

func (r *BusRepository) CreateBusWithSeats(ctx context.Context, bus *models.Bus, seats []models.Seat) error {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		log.Println("[CreateBusWithSeats] begin tx error:", err)
		return err
	}
	defer tx.Rollback(ctx)

	busQuery := `
		insert into buses (id, name, total_seats, columns_left, columns_right, created_at)
		values ($1, $2, $3, $4, $5, $6)
	`
	_, err = tx.Exec(ctx, busQuery,
		bus.ID, bus.Name, bus.TotalSeats,
		bus.ColumnsLeft, bus.ColumnsRight, bus.CreatedAt,
	)
	if err != nil {
		log.Println("[CreateBusWithSeats] insert bus error:", err)
		return err
	}

	seatQuery := `
		insert into seats (id, bus_id, seat_number, seat_row, seat_column, created_at)
		values ($1, $2, $3, $4, $5, $6)
	`
	for _, seat := range seats {
		_, err = tx.Exec(ctx, seatQuery,
			seat.ID, seat.BusID, seat.SeatNumber,
			seat.SeatRow, seat.SeatColumn, seat.CreatedAt,
		)
		if err != nil {
			log.Println("[CreateBusWithSeats] insert seat error:", err)
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *BusRepository) GetSeatByID(ctx context.Context, seatID uuid.UUID) (*models.Seat, error) {
	query := `
		select id, bus_id, seat_number, seat_row, seat_column, created_at
		from seats where id = $1
	`
	row := r.DB.QueryRow(ctx, query, seatID)
	var s models.Seat
	err := row.Scan(&s.ID, &s.BusID, &s.SeatNumber, &s.SeatRow, &s.SeatColumn, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *BusRepository) GetSeatByNumber(ctx context.Context, busID uuid.UUID, seatNumber string) (*models.Seat, error) {
	query := `
		select id, bus_id, seat_number, seat_row, seat_column, created_at
		from seats where bus_id = $1 and seat_number = $2
	`
	row := r.DB.QueryRow(ctx, query, busID, seatNumber)
	var s models.Seat
	err := row.Scan(&s.ID, &s.BusID, &s.SeatNumber, &s.SeatRow, &s.SeatColumn, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *BusRepository) UpdateSeat(ctx context.Context, seatID uuid.UUID, seatNumber string) (*models.Seat, error) {
	query := `
		update seats
		set seat_number = $1
		where id = $2
		returning id, bus_id, seat_number, seat_row, seat_column, created_at
	`
	row := r.DB.QueryRow(ctx, query, seatNumber, seatID)
	var s models.Seat
	err := row.Scan(&s.ID, &s.BusID, &s.SeatNumber, &s.SeatRow, &s.SeatColumn, &s.CreatedAt)
	if err != nil {
		log.Println("[UpdateSeat] error:", err)
		return nil, err
	}
	return &s, nil
}
