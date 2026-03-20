package models

import (
    "time"
    "github.com/google/uuid"
)

type Booking struct {
    ID         uuid.UUID  `db:"id" json:"id"`
    UserID     uuid.UUID  `db:"user_id" json:"user_id"`
    TripID     uuid.UUID  `db:"trip_id" json:"trip_id"`
    SeatID     *uuid.UUID `db:"seat_id" json:"seat_id,omitempty"`
    FromStopID *uuid.UUID `db:"from_stop_id" json:"from_stop_id,omitempty"`
    ToStopID   *uuid.UUID `db:"to_stop_id" json:"to_stop_id,omitempty"`
    Status     string     `db:"status" json:"status"`
    CreatedAt  time.Time  `db:"created_at" json:"created_at"`
}
