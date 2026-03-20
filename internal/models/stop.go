package models

import (
    "time"
    "github.com/google/uuid"
)

type Stop struct {
    ID        uuid.UUID `db:"id" json:"id"`
    Name      string    `db:"name" json:"name"`
    City      *string   `db:"city" json:"city,omitempty"`
    CreatedAt time.Time `db:"created_at" json:"created_at"`
}
