package service

import (
	"context"
	"errors"
	"fmt"
	"tayo-booking/internal/models"
	"tayo-booking/internal/repository"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type BusService struct {
	BusRepo *repository.BusRepository
}

func NewBusService(busRepo *repository.BusRepository) *BusService {
	return &BusService{BusRepo: busRepo}
}

func (s *BusService) CreateBus(ctx context.Context, name string, totalRows, columnsLeft, columnsRight int) (*models.Bus, error) {
	totalSeats := totalRows * (columnsLeft + columnsRight)
	now := time.Now()

	bus := &models.Bus{
		ID:           uuid.New(),
		Name:         name,
		TotalSeats:   totalSeats,
		ColumnsLeft:  columnsLeft,
		ColumnsRight: columnsRight,
		CreatedAt:    now,
	}

	seats := make([]models.Seat, 0, totalSeats)
	for row := 0; row < totalRows; row++ {
		rowLetter := string(rune('A' + row))
		totalCols := columnsLeft + columnsRight
		for col := 1; col <= totalCols; col++ {
			seatRow := rowLetter
			seatCol := col
			seatNumber := fmt.Sprintf("%s%d", rowLetter, col)
			seats = append(seats, models.Seat{
				ID:         uuid.New(),
				BusID:      bus.ID,
				SeatNumber: seatNumber,
				SeatRow:    &seatRow,
				SeatColumn: &seatCol,
				CreatedAt:  now,
			})
		}
	}

	if err := s.BusRepo.CreateBusWithSeats(ctx, bus, seats); err != nil {
		return nil, errors.New("failed to create bus")
	}
	return bus, nil
}

func (s *BusService) UpdateSeat(ctx context.Context, seatID uuid.UUID, seatNumber string) (*models.Seat, error) {
	// Check the seat exists
	existing, err := s.BusRepo.GetSeatByID(ctx, seatID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("seat not found")
		}
		return nil, errors.New("failed to update seat")
	}

	// Check duplicate seat_number on the same bus
	duplicate, err := s.BusRepo.GetSeatByNumber(ctx, existing.BusID, seatNumber)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("failed to update seat")
	}
	if duplicate != nil && duplicate.ID != seatID {
		return nil, errors.New("seat number already exists on this bus")
	}

	seat, err := s.BusRepo.UpdateSeat(ctx, seatID, seatNumber)
	if err != nil {
		return nil, errors.New("failed to update seat")
	}
	return seat, nil
}
