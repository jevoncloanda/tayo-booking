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
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	// The seat map needs the aisle position, so the column split travels with
	// every bus payload rather than only with the admin bus list.
	TotalSeats   int `json:"total_seats"`
	ColumnsLeft  int `json:"columns_left"`
	ColumnsRight int `json:"columns_right"`
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

// StopEntry is one ordered stop on a route, shared by the trip detail and
// route list responses.
type StopEntry struct {
	StopOrder int      `json:"stop_order"`
	Stop      StopInfo `json:"stop"`
}

type TripDetail struct {
	ID            uuid.UUID   `json:"id"`
	Route         RouteInfo   `json:"route"`
	Bus           BusInfo     `json:"bus"`
	DepartureTime time.Time   `json:"departure_time"`
	ArrivalTime   time.Time   `json:"arrival_time"`
	Price         float64     `json:"price"`
	Stops         []StopEntry `json:"stops"`
}
