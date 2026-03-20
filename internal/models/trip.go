package models

import (
    "time"
    "github.com/google/uuid"
)

type Trip struct {
    ID            uuid.UUID `db:"id" json:"id"`
    RouteID       uuid.UUID `db:"route_id" json:"route_id"`
    BusID         uuid.UUID `db:"bus_id" json:"bus_id"`
    DepartureTime time.Time `db:"departure_time" json:"departure_time"`
    ArrivalTime   time.Time `db:"arrival_time" json:"arrival_time"`
    Price         float64   `db:"price" json:"price"`
    CreatedAt     time.Time `db:"created_at" json:"created_at"`
}
