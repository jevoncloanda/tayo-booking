package models

import (
	"time"

	"github.com/google/uuid"
)

type RouteInfo struct {
	ID   uuid.UUID `json:"id"`
	Name *string   `json:"name"`
}

type BusInfo struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	TotalSeats int       `json:"total_seats"`
}

type StopInfo struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	City *string   `json:"city,omitempty"`
}

// Used in GET /trips list response
type TripSummary struct {
	ID            uuid.UUID `json:"id"`
	Route         RouteInfo `json:"route"`
	Bus           BusInfo   `json:"bus"`
	DepartureTime time.Time `json:"departure_time"`
	ArrivalTime   time.Time `json:"arrival_time"`
	Price         float64   `json:"price"`
	FromStop      *StopInfo `json:"from_stop,omitempty"`
	ToStop        *StopInfo `json:"to_stop,omitempty"`
}

// Used in GET /trips/:id response
type TripStopEntry struct {
	StopOrder int      `json:"stop_order"`
	Stop      StopInfo `json:"stop"`
}

type TripDetail struct {
	ID            uuid.UUID       `json:"id"`
	Route         RouteInfo       `json:"route"`
	Bus           BusInfo         `json:"bus"`
	DepartureTime time.Time       `json:"departure_time"`
	ArrivalTime   time.Time       `json:"arrival_time"`
	Price         float64         `json:"price"`
	Stops         []TripStopEntry `json:"stops"`
}
