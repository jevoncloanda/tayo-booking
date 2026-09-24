package models

import (
	"github.com/google/uuid"
	"time"
)

type Route struct {
	ID        uuid.UUID `db:"id" json:"id"`
	Name      *string   `db:"name" json:"name,omitempty"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// RouteDetail is a route with its ordered stops, returned by GET /admin/routes.
type RouteDetail struct {
	ID        uuid.UUID   `json:"id"`
	Name      *string     `json:"name,omitempty"`
	CreatedAt time.Time   `json:"created_at"`
	Stops     []StopEntry `json:"stops"`
}
