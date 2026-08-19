package models

import (
	"time"

	"github.com/google/uuid"
)

// Booking is the raw DB row.
type Booking struct {
	ID         uuid.UUID  `db:"id"           json:"id"`
	UserID     uuid.UUID  `db:"user_id"      json:"user_id"`
	TripID     uuid.UUID  `db:"trip_id"      json:"trip_id"`
	SeatID     *uuid.UUID `db:"seat_id"      json:"seat_id,omitempty"`
	FromStopID *uuid.UUID `db:"from_stop_id" json:"from_stop_id,omitempty"`
	ToStopID   *uuid.UUID `db:"to_stop_id"   json:"to_stop_id,omitempty"`
	Status     string     `db:"status"       json:"status"`
	CreatedAt  time.Time  `db:"created_at"   json:"created_at"`
}

// BookingTripInfo is the nested trip object in booking responses.
type BookingTripInfo struct {
	ID            uuid.UUID `json:"id"`
	Route         RouteInfo `json:"route"`
	DepartureTime time.Time `json:"departure_time"`
	ArrivalTime   time.Time `json:"arrival_time"`
	Price         float64   `json:"price"`
}

// BookingSeatInfo is the nested seat object in booking responses.
type BookingSeatInfo struct {
	ID         uuid.UUID `json:"id"`
	SeatNumber string    `json:"seat_number"`
}

// BookingUserInfo is the nested user object in admin booking responses.
type BookingUserInfo struct {
	ID    uuid.UUID `json:"id"`
	Name  *string   `json:"name"`
	Email string    `json:"email"`
}

// BookingDetail is the enriched booking response for the authenticated user.
type BookingDetail struct {
	ID        uuid.UUID       `json:"id"`
	Trip      BookingTripInfo `json:"trip"`
	Seat      BookingSeatInfo `json:"seat"`
	FromStop  StopInfo        `json:"from_stop"`
	ToStop    StopInfo        `json:"to_stop"`
	Status    string          `json:"status"`
	CreatedAt time.Time       `json:"created_at"`
}

// BookingDetailAdmin extends BookingDetail with a user field for admin views.
type BookingDetailAdmin struct {
	ID        uuid.UUID       `json:"id"`
	User      BookingUserInfo `json:"user"`
	Trip      BookingTripInfo `json:"trip"`
	Seat      BookingSeatInfo `json:"seat"`
	FromStop  StopInfo        `json:"from_stop"`
	ToStop    StopInfo        `json:"to_stop"`
	Status    string          `json:"status"`
	CreatedAt time.Time       `json:"created_at"`
}
