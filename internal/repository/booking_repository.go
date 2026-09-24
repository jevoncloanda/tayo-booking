package repository

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"tayo-booking/internal/domain"
	"tayo-booking/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type BookingRepository struct {
	DB *pgxpool.Pool
	// testSchema is set only by the isolated database integration test.
	testSchema string
}

func NewBookingRepository(db *pgxpool.Pool) *BookingRepository {
	return &BookingRepository{DB: db}
}

// CreateBooking serializes duplicate requests and competing seat claims.
// replayed is true when an earlier successful request is returned.
func (r *BookingRepository) CreateBooking(
	ctx context.Context,
	booking *models.Booking,
	idempotencyKey string,
	requestHash []byte,
) (replayed bool, err error) {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin booking transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	if r.testSchema != "" {
		identifier := pgx.Identifier{r.testSchema}.Sanitize()
		if _, err := tx.Exec(ctx, "set local search_path to "+identifier); err != nil {
			return false, fmt.Errorf("set booking test schema: %w", err)
		}
		var activeSchema string
		if err := tx.QueryRow(ctx, `select current_schema()`).Scan(&activeSchema); err != nil {
			return false, fmt.Errorf("verify booking test schema: %w", err)
		}
		if activeSchema != r.testSchema {
			return false, fmt.Errorf("booking test schema mismatch: got %q", activeSchema)
		}
	}

	// Every create path takes locks in this order to avoid deadlocks:
	// logical request first, then physical trip seat.
	if _, err := tx.Exec(ctx, `
		select pg_advisory_xact_lock(
			hashtext('booking-idempotency:' || $1::text),
			hashtext($2)
		)
	`, booking.UserID, idempotencyKey); err != nil {
		return false, fmt.Errorf("lock booking idempotency key: %w", err)
	}

	var storedHash []byte
	err = tx.QueryRow(ctx, `
		select i.request_hash,
		       b.id, b.user_id, b.trip_id, b.seat_id,
		       b.from_stop_id, b.to_stop_id, 'pending', b.created_at
		from booking_idempotency i
		join bookings b on b.id = i.booking_id
		where i.user_id = $1 and i.idempotency_key = $2
	`, booking.UserID, idempotencyKey).Scan(
		&storedHash,
		&booking.ID, &booking.UserID, &booking.TripID, &booking.SeatID,
		&booking.FromStopID, &booking.ToStopID, &booking.Status, &booking.CreatedAt,
	)
	switch {
	case err == nil:
		if !bytes.Equal(storedHash, requestHash) {
			log.Printf("[booking] idempotency conflict user_id=%s", booking.UserID)
			return false, domain.ErrIdempotencyConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return false, fmt.Errorf("commit idempotent booking replay: %w", err)
		}
		log.Printf("[booking] idempotent replay booking_id=%s user_id=%s", booking.ID, booking.UserID)
		return true, nil
	case !errors.Is(err, pgx.ErrNoRows):
		return false, fmt.Errorf("read booking idempotency result: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		select pg_advisory_xact_lock(
			hashtext('booking-seat:' || $1::text),
			hashtext($2::text)
		)
	`, booking.TripID, booking.SeatID); err != nil {
		return false, fmt.Errorf("lock trip seat: %w", err)
	}

	fromOrder, toOrder, err := validateStopsOnRoute(ctx, tx, booking.TripID, *booking.FromStopID, *booking.ToStopID)
	if err != nil {
		return false, err
	}
	if fromOrder >= toOrder {
		return false, domain.ErrInvalidSegment
	}
	if err := validateSeatOnBus(ctx, tx, booking.TripID, *booking.SeatID); err != nil {
		return false, err
	}

	// CTE-based availability check — mirrors GetSeatsForSegment pattern.
	checkQuery := `
		with trip_route as (
			select t.bus_id, t.route_id
			from trips t
			where t.id = $1
		),
		from_order as (
			select rs.stop_order
			from route_stops rs
			join trip_route tr on rs.route_id = tr.route_id
			where rs.stop_id = $3
		),
		to_order as (
			select rs.stop_order
			from route_stops rs
			join trip_route tr on rs.route_id = tr.route_id
			where rs.stop_id = $4
		),
		booked as (
			select 1
			from bookings b
			join route_stops bfrs on bfrs.stop_id = b.from_stop_id
				and bfrs.route_id = (select route_id from trip_route)
			join route_stops btrs on btrs.stop_id = b.to_stop_id
				and btrs.route_id = (select route_id from trip_route)
			where b.trip_id  = $1
			  and b.seat_id  = $2
			  and b.status in ('confirmed', 'pending')
			  and bfrs.stop_order < (select stop_order from to_order)
			  and btrs.stop_order > (select stop_order from from_order)
		)
		select exists (select 1 from booked)
	`
	var taken bool
	err = tx.QueryRow(ctx, checkQuery, booking.TripID, booking.SeatID, booking.FromStopID, booking.ToStopID).Scan(&taken)
	if err != nil {
		return false, fmt.Errorf("check seat availability: %w", err)
	}
	if taken {
		log.Printf("[booking] seat conflict trip_id=%s seat_id=%s", booking.TripID, *booking.SeatID)
		return false, domain.ErrSeatUnavailable
	}

	insertQuery := `
		insert into bookings (id, user_id, trip_id, seat_id, from_stop_id, to_stop_id, status, created_at)
		values ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err = tx.Exec(ctx, insertQuery,
		booking.ID,
		booking.UserID,
		booking.TripID,
		booking.SeatID,
		booking.FromStopID,
		booking.ToStopID,
		booking.Status,
		booking.CreatedAt,
	)
	if err != nil {
		return false, fmt.Errorf("insert booking: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		insert into booking_idempotency (user_id, idempotency_key, request_hash, booking_id)
		values ($1, $2, $3, $4)
	`, booking.UserID, idempotencyKey, requestHash, booking.ID); err != nil {
		return false, fmt.Errorf("store booking idempotency result: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit booking: %w", err)
	}
	log.Printf("[booking] created booking_id=%s user_id=%s", booking.ID, booking.UserID)
	return false, nil
}

const bookingDetailSelect = `
	select
		b.id,
		b.status,
		b.created_at,
		t.id, t.departure_time, t.arrival_time, t.price,
		r.id, r.name,
		s.id, s.seat_number,
		fs.id, fs.name, fs.city,
		ts.id, ts.name, ts.city
	from bookings b
	join trips   t  on t.id  = b.trip_id
	join routes  r  on r.id  = t.route_id
	join seats   s  on s.id  = b.seat_id
	join stops   fs on fs.id = b.from_stop_id
	join stops   ts on ts.id = b.to_stop_id
`

// rowScanner is satisfied by both pgx.Row and pgx.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanBookingDetail(row rowScanner) (*models.BookingDetail, error) {
	var d models.BookingDetail
	err := row.Scan(
		&d.ID, &d.Status, &d.CreatedAt,
		&d.Trip.ID, &d.Trip.DepartureTime, &d.Trip.ArrivalTime, &d.Trip.Price,
		&d.Trip.Route.ID, &d.Trip.Route.Name,
		&d.Seat.ID, &d.Seat.SeatNumber,
		&d.FromStop.ID, &d.FromStop.Name, &d.FromStop.City,
		&d.ToStop.ID, &d.ToStop.Name, &d.ToStop.City,
	)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// GetBookingsByUser returns all bookings for a user with full join data.
func (r *BookingRepository) GetBookingsByUser(ctx context.Context, userID uuid.UUID) ([]models.BookingDetail, error) {
	query := bookingDetailSelect + `
		where b.user_id = $1
		order by b.created_at desc
	`
	rows, err := r.DB.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.BookingDetail
	for rows.Next() {
		d, err := scanBookingDetail(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, *d)
	}
	if results == nil {
		results = []models.BookingDetail{}
	}
	return results, nil
}

// GetBookingByID returns a single booking detail row.
func (r *BookingRepository) GetBookingByID(ctx context.Context, id uuid.UUID) (*models.BookingDetail, error) {
	query := bookingDetailSelect + `where b.id = $1`
	row := r.DB.QueryRow(ctx, query, id)
	d, err := scanBookingDetail(row)
	if err != nil {
		return nil, err
	}
	return d, nil
}

// GetBookingOwner returns only the user_id for a booking — used for ownership checks.
func (r *BookingRepository) GetBookingOwner(ctx context.Context, id uuid.UUID) (uuid.UUID, string, error) {
	query := `
		select b.user_id, b.status
		from bookings b
		where b.id = $1
	`
	var ownerID uuid.UUID
	var status string
	err := r.DB.QueryRow(ctx, query, id).Scan(&ownerID, &status)
	if err != nil {
		return uuid.Nil, "", err
	}
	return ownerID, status, nil
}

// GetTripForBookingCancellation returns trip departure_time and max_cancellation_minutes.
func (r *BookingRepository) GetTripForBookingCancellation(ctx context.Context, bookingID uuid.UUID) (models.Trip, error) {
	query := `
		select t.id, t.route_id, t.bus_id, t.departure_time, t.arrival_time,
		       t.price, t.max_cancellation_minutes, t.created_at
		from bookings b
		join trips t on t.id = b.trip_id
		where b.id = $1
	`
	var trip models.Trip
	err := r.DB.QueryRow(ctx, query, bookingID).Scan(
		&trip.ID, &trip.RouteID, &trip.BusID,
		&trip.DepartureTime, &trip.ArrivalTime,
		&trip.Price, &trip.MaxCancellationMinutes, &trip.CreatedAt,
	)
	if err != nil {
		return models.Trip{}, err
	}
	return trip, nil
}

// CancelBooking sets a booking's status to cancelled.
func (r *BookingRepository) CancelBooking(ctx context.Context, id uuid.UUID) error {
	query := `update bookings set status = 'cancelled' where id = $1`
	_, err := r.DB.Exec(ctx, query, id)
	if err != nil {
		log.Println("[CancelBooking] error:", err)
	}
	return err
}

// BookingFilter holds optional filters for admin booking list.
type BookingFilter struct {
	Status *string
	TripID *uuid.UUID
}

const bookingDetailAdminSelect = `
	select
		b.id,
		b.status,
		b.created_at,
		u.id, u.name, u.email,
		t.id, t.departure_time, t.arrival_time, t.price,
		r.id, r.name,
		s.id, s.seat_number,
		fs.id, fs.name, fs.city,
		ts.id, ts.name, ts.city
	from bookings b
	join users   u  on u.id  = b.user_id
	join trips   t  on t.id  = b.trip_id
	join routes  r  on r.id  = t.route_id
	join seats   s  on s.id  = b.seat_id
	join stops   fs on fs.id = b.from_stop_id
	join stops   ts on ts.id = b.to_stop_id
`

func scanBookingDetailAdmin(rows rowScanner) (*models.BookingDetailAdmin, error) {
	var d models.BookingDetailAdmin
	err := rows.Scan(
		&d.ID, &d.Status, &d.CreatedAt,
		&d.User.ID, &d.User.Name, &d.User.Email,
		&d.Trip.ID, &d.Trip.DepartureTime, &d.Trip.ArrivalTime, &d.Trip.Price,
		&d.Trip.Route.ID, &d.Trip.Route.Name,
		&d.Seat.ID, &d.Seat.SeatNumber,
		&d.FromStop.ID, &d.FromStop.Name, &d.FromStop.City,
		&d.ToStop.ID, &d.ToStop.Name, &d.ToStop.City,
	)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// GetAllBookings returns all bookings for admin with optional filters.
func (r *BookingRepository) GetAllBookings(ctx context.Context, f BookingFilter) ([]models.BookingDetailAdmin, error) {
	args := []any{}
	argIdx := 1
	where := ""

	if f.Status != nil {
		where += fmt.Sprintf(" where b.status = $%d", argIdx)
		args = append(args, *f.Status)
		argIdx++
		if f.TripID != nil {
			where += fmt.Sprintf(" and b.trip_id = $%d", argIdx)
			args = append(args, *f.TripID)
			argIdx++
		}
	} else if f.TripID != nil {
		where += fmt.Sprintf(" where b.trip_id = $%d", argIdx)
		args = append(args, *f.TripID)
		argIdx++
	}

	query := bookingDetailAdminSelect + where + " order by b.created_at desc"

	rows, err := r.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.BookingDetailAdmin
	for rows.Next() {
		d, err := scanBookingDetailAdmin(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, *d)
	}
	if results == nil {
		results = []models.BookingDetailAdmin{}
	}
	return results, nil
}

// GetAdminBookingByID returns a single booking detail with user info for admin.
func (r *BookingRepository) GetAdminBookingByID(ctx context.Context, id uuid.UUID) (*models.BookingDetailAdmin, error) {
	query := bookingDetailAdminSelect + " where b.id = $1"
	row := r.DB.QueryRow(ctx, query, id)
	return scanBookingDetailAdmin(row)
}

// UpdateBookingStatus sets the status of a booking by ID.
func (r *BookingRepository) UpdateBookingStatus(ctx context.Context, id uuid.UUID, status string) error {
	query := `update bookings set status = $1 where id = $2`
	_, err := r.DB.Exec(ctx, query, status, id)
	if err != nil {
		log.Println("[UpdateBookingStatus] error:", err)
	}
	return err
}

// ValidateStopsOnRoute checks that both stops belong to the trip's route
// and returns their stop_orders (fromOrder, toOrder).
func (r *BookingRepository) ValidateStopsOnRoute(ctx context.Context, tripID, fromStopID, toStopID uuid.UUID) (int, int, error) {
	return validateStopsOnRoute(ctx, r.DB, tripID, fromStopID, toStopID)
}

type queryRower interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func validateStopsOnRoute(ctx context.Context, db queryRower, tripID, fromStopID, toStopID uuid.UUID) (int, int, error) {
	query := `
		with trip_route as (
			select route_id from trips where id = $1
		),
		from_rs as (
			select rs.stop_order
			from route_stops rs
			join trip_route tr on rs.route_id = tr.route_id
			where rs.stop_id = $2
		),
		to_rs as (
			select rs.stop_order
			from route_stops rs
			join trip_route tr on rs.route_id = tr.route_id
			where rs.stop_id = $3
		)
		select
			(select stop_order from from_rs),
			(select stop_order from to_rs)
	`
	var fromOrder, toOrder *int
	err := db.QueryRow(ctx, query, tripID, fromStopID, toStopID).Scan(&fromOrder, &toOrder)
	if err != nil {
		return 0, 0, err
	}
	if fromOrder == nil {
		return 0, 0, domain.ErrFromStopNotOnRoute
	}
	if toOrder == nil {
		return 0, 0, domain.ErrToStopNotOnRoute
	}
	return *fromOrder, *toOrder, nil
}

// ValidateSeatOnBus checks that a seat belongs to the trip's bus.
func (r *BookingRepository) ValidateSeatOnBus(ctx context.Context, tripID, seatID uuid.UUID) error {
	return validateSeatOnBus(ctx, r.DB, tripID, seatID)
}

func validateSeatOnBus(ctx context.Context, db queryRower, tripID, seatID uuid.UUID) error {
	query := `
		select 1
		from seats s
		join trips t on t.bus_id = s.bus_id
		where t.id = $1 and s.id = $2
	`
	var dummy int
	err := db.QueryRow(ctx, query, tripID, seatID).Scan(&dummy)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrSeatNotOnBus
		}
		return err
	}
	return nil
}
