package models

import (
    "time"
    "github.com/google/uuid"
)

type RouteStop struct {
    ID        uuid.UUID `db:"id" json:"id"`
    RouteID   uuid.UUID `db:"route_id" json:"route_id"`
    StopID    uuid.UUID `db:"stop_id" json:"stop_id"`
    StopOrder int       `db:"stop_order" json:"stop_order"`
    CreatedAt time.Time `db:"created_at" json:"created_at"`
}
