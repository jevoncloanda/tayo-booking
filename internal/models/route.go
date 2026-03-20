package models

import (
    "time"
    "github.com/google/uuid"
)

type Route struct {
    ID        uuid.UUID `db:"id" json:"id"`
    Name      *string   `db:"name" json:"name,omitempty"`
    CreatedAt time.Time `db:"created_at" json:"created_at"`
}
