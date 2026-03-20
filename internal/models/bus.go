package models

import (
    "time"
    "github.com/google/uuid"
)

type Bus struct {
    ID         uuid.UUID `db:"id" json:"id"`
    Name       string    `db:"name" json:"name"`
    TotalSeats int       `db:"total_seats" json:"total_seats"`
    CreatedAt  time.Time `db:"created_at" json:"created_at"`
}
