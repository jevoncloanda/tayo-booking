package models

import (
	"time"

	"github.com/google/uuid"
)

type Seat struct {
	ID         uuid.UUID `db:"id"          json:"id"`
	BusID      uuid.UUID `db:"bus_id"      json:"bus_id"`
	SeatNumber string    `db:"seat_number" json:"seat_number"`
	SeatRow    *string   `db:"seat_row"    json:"seat_row"`
	SeatColumn *int      `db:"seat_column" json:"seat_column"`
	CreatedAt  time.Time `db:"created_at"  json:"created_at"`
}
