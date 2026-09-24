package repository

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"tayo-booking/internal/domain"
	"tayo-booking/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

type bookingFixture struct {
	pool                  *pgxpool.Pool
	schema                string
	user1, user2          uuid.UUID
	trip, seat, otherSeat uuid.UUID
	a, b, c, d, outside   uuid.UUID
}

func TestBookingCreationIntegration(t *testing.T) {
	fx := newBookingFixture(t)
	repo := NewBookingRepository(fx.pool)
	repo.testSchema = fx.schema

	t.Run("segment overlap", func(t *testing.T) {
		tests := []struct {
			name                       string
			existingFrom, existingTo   uuid.UUID
			requestedFrom, requestedTo uuid.UUID
			wantConflict               bool
		}{
			{"adjacent A-B and B-D", fx.a, fx.b, fx.b, fx.d, false},
			{"overlap A-C and B-D", fx.a, fx.c, fx.b, fx.d, true},
			{"containing A-D and B-C", fx.a, fx.d, fx.b, fx.c, true},
			{"contained B-C and A-D", fx.b, fx.c, fx.a, fx.d, true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				clearBookings(t, fx)
				insertExistingBooking(t, fx, tt.existingFrom, tt.existingTo)

				booking := newTestBooking(fx.user2, fx.trip, fx.seat, tt.requestedFrom, tt.requestedTo)
				_, err := repo.CreateBooking(context.Background(), booking, uuid.NewString(), bookingHash(booking))
				if tt.wantConflict && !errors.Is(err, domain.ErrSeatUnavailable) {
					t.Fatalf("expected seat conflict, got %v", err)
				}
				if !tt.wantConflict && err != nil {
					t.Fatalf("expected segment reuse, got %v", err)
				}
			})
		}
	})

	t.Run("invalid input", func(t *testing.T) {
		tests := []struct {
			name     string
			from, to uuid.UUID
			seat     uuid.UUID
			want     error
		}{
			{"origin not on route", fx.outside, fx.d, fx.seat, domain.ErrFromStopNotOnRoute},
			{"destination not on route", fx.a, fx.outside, fx.seat, domain.ErrToStopNotOnRoute},
			{"destination before origin", fx.d, fx.b, fx.seat, domain.ErrInvalidSegment},
			{"seat not on trip bus", fx.a, fx.b, fx.otherSeat, domain.ErrSeatNotOnBus},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				clearBookings(t, fx)
				booking := newTestBooking(fx.user1, fx.trip, tt.seat, tt.from, tt.to)
				_, err := repo.CreateBooking(context.Background(), booking, uuid.NewString(), bookingHash(booking))
				if !errors.Is(err, tt.want) {
					t.Fatalf("expected %v, got %v", tt.want, err)
				}
			})
		}
	})

	t.Run("concurrent overlapping requests allow exactly one", func(t *testing.T) {
		clearBookings(t, fx)
		first := newTestBooking(fx.user1, fx.trip, fx.seat, fx.a, fx.c)
		second := newTestBooking(fx.user2, fx.trip, fx.seat, fx.b, fx.d)
		results := runConcurrentCreates(repo,
			createAttempt{first, "overlap-1", bookingHash(first)},
			createAttempt{second, "overlap-2", bookingHash(second)},
		)

		var successes, conflicts int
		for _, result := range results {
			switch {
			case result.err == nil:
				successes++
			case errors.Is(result.err, domain.ErrSeatUnavailable):
				conflicts++
			default:
				t.Fatalf("unexpected concurrent result: %v", result.err)
			}
		}
		if successes != 1 || conflicts != 1 {
			t.Fatalf("expected one success and one conflict, got %d successes and %d conflicts", successes, conflicts)
		}
		assertBookingCount(t, fx, 1)
	})

	t.Run("concurrent non-overlapping requests both succeed", func(t *testing.T) {
		clearBookings(t, fx)
		first := newTestBooking(fx.user1, fx.trip, fx.seat, fx.a, fx.b)
		second := newTestBooking(fx.user2, fx.trip, fx.seat, fx.b, fx.d)
		results := runConcurrentCreates(repo,
			createAttempt{first, "adjacent-1", bookingHash(first)},
			createAttempt{second, "adjacent-2", bookingHash(second)},
		)
		for _, result := range results {
			if result.err != nil {
				t.Fatalf("expected both adjacent segments to succeed, got %v", result.err)
			}
		}
		assertBookingCount(t, fx, 2)
	})

	t.Run("idempotent replay", func(t *testing.T) {
		clearBookings(t, fx)
		key := "same-logical-request"
		first := newTestBooking(fx.user1, fx.trip, fx.seat, fx.a, fx.b)
		replayed, err := repo.CreateBooking(context.Background(), first, key, bookingHash(first))
		if err != nil || replayed {
			t.Fatalf("first request: replayed=%v err=%v", replayed, err)
		}
		if _, err := fx.pool.Exec(context.Background(), "update "+fx.table("bookings")+" set status = 'confirmed' where id = $1", first.ID); err != nil {
			t.Fatal(err)
		}

		second := newTestBooking(fx.user1, fx.trip, fx.seat, fx.a, fx.b)
		replayed, err = repo.CreateBooking(context.Background(), second, key, bookingHash(second))
		if err != nil || !replayed {
			t.Fatalf("retry: replayed=%v err=%v", replayed, err)
		}
		if first.ID != second.ID {
			t.Fatalf("expected replayed booking %s, got %s", first.ID, second.ID)
		}
		if second.Status != "pending" {
			t.Fatalf("expected original pending response, got %s", second.Status)
		}
		assertBookingCount(t, fx, 1)
	})

	t.Run("same key with different payload conflicts", func(t *testing.T) {
		clearBookings(t, fx)
		key := "misused-key"
		first := newTestBooking(fx.user1, fx.trip, fx.seat, fx.a, fx.b)
		if _, err := repo.CreateBooking(context.Background(), first, key, bookingHash(first)); err != nil {
			t.Fatal(err)
		}
		second := newTestBooking(fx.user1, fx.trip, fx.seat, fx.a, fx.c)
		_, err := repo.CreateBooking(context.Background(), second, key, bookingHash(second))
		if !errors.Is(err, domain.ErrIdempotencyConflict) {
			t.Fatalf("expected idempotency conflict, got %v", err)
		}
		assertBookingCount(t, fx, 1)
	})

	t.Run("concurrent same key creates one booking", func(t *testing.T) {
		clearBookings(t, fx)
		first := newTestBooking(fx.user1, fx.trip, fx.seat, fx.a, fx.b)
		second := newTestBooking(fx.user1, fx.trip, fx.seat, fx.a, fx.b)
		results := runConcurrentCreates(repo,
			createAttempt{first, "concurrent-retry", bookingHash(first)},
			createAttempt{second, "concurrent-retry", bookingHash(second)},
		)

		var replayed int
		for _, result := range results {
			if result.err != nil {
				t.Fatalf("expected successful replay, got %v", result.err)
			}
			if result.replayed {
				replayed++
			}
		}
		if replayed != 1 || first.ID != second.ID {
			t.Fatalf("expected one replay of one booking, replayed=%d ids=%s,%s", replayed, first.ID, second.ID)
		}
		assertBookingCount(t, fx, 1)
	})
}

type createAttempt struct {
	booking *models.Booking
	key     string
	hash    []byte
}

type createResult struct {
	replayed bool
	err      error
}

func runConcurrentCreates(repo *BookingRepository, attempts ...createAttempt) []createResult {
	start := make(chan struct{})
	results := make(chan createResult, len(attempts))
	for _, attempt := range attempts {
		attempt := attempt
		go func() {
			<-start
			replayed, err := repo.CreateBooking(context.Background(), attempt.booking, attempt.key, attempt.hash)
			results <- createResult{replayed, err}
		}()
	}
	close(start)

	collected := make([]createResult, 0, len(attempts))
	for range attempts {
		collected = append(collected, <-results)
	}
	return collected
}

func newTestBooking(userID, tripID, seatID, fromStopID, toStopID uuid.UUID) *models.Booking {
	return &models.Booking{
		ID: uuid.New(), UserID: userID, TripID: tripID,
		SeatID: &seatID, FromStopID: &fromStopID, ToStopID: &toStopID,
		Status: "pending", CreatedAt: time.Now().UTC(),
	}
}

func bookingHash(booking *models.Booking) []byte {
	sum := sha256.Sum256([]byte(
		"booking:v1\x00" + booking.TripID.String() + "\x00" + booking.SeatID.String() + "\x00" +
			booking.FromStopID.String() + "\x00" + booking.ToStopID.String(),
	))
	return sum[:]
}

func newBookingFixture(t *testing.T) bookingFixture {
	t.Helper()
	_ = godotenv.Load("../../.env.test")
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL booking integration tests")
	}

	ctx := context.Background()
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	adminPool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	schema := "tayo_booking_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	identifier := pgx.Identifier{schema}.Sanitize()
	if _, err := adminPool.Exec(ctx, "create schema "+identifier); err != nil {
		adminPool.Close()
		t.Fatal(err)
	}

	var pool *pgxpool.Pool
	t.Cleanup(func() {
		if pool != nil {
			pool.Close()
		}
		defer adminPool.Close()
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		tx, err := adminPool.Begin(cleanupCtx)
		if err != nil {
			t.Errorf("begin temporary schema cleanup: %v", err)
			return
		}
		defer tx.Rollback(cleanupCtx)
		if _, err := tx.Exec(cleanupCtx, "set local search_path to "+identifier); err != nil {
			t.Errorf("set cleanup search path: %v", err)
			return
		}
		var activeSchema, searchPath string
		if err := tx.QueryRow(cleanupCtx, `select current_schema(), current_setting('search_path')`).Scan(&activeSchema, &searchPath); err != nil {
			t.Errorf("verify cleanup search path: %v", err)
			return
		}
		if activeSchema != schema || searchPath != schema {
			t.Errorf("cleanup refused: active schema %q, search path %q, target %q", activeSchema, searchPath, schema)
			return
		}
		if _, err := tx.Exec(cleanupCtx, "drop schema "+identifier+" cascade"); err != nil {
			t.Errorf("drop exact temporary schema %q: %v", schema, err)
			return
		}
		if err := tx.Commit(cleanupCtx); err != nil {
			t.Errorf("commit temporary schema cleanup: %v", err)
		}
	})
	pool, err = pgxpool.NewWithConfig(ctx, config.Copy())
	if err != nil {
		t.Fatal(err)
	}

	for _, statement := range testSchemaStatements {
		statement = strings.ReplaceAll(statement, "{schema}.", identifier+".")
		if _, err := pool.Exec(ctx, statement); err != nil {
			t.Fatalf("create test schema: %v", err)
		}
	}

	fx := bookingFixture{
		pool: pool, schema: schema, user1: uuid.New(), user2: uuid.New(), trip: uuid.New(),
		seat: uuid.New(), otherSeat: uuid.New(), a: uuid.New(), b: uuid.New(),
		c: uuid.New(), d: uuid.New(), outside: uuid.New(),
	}
	seedFixture(t, fx)
	return fx
}

func (fx bookingFixture) table(name string) string {
	return pgx.Identifier{fx.schema, name}.Sanitize()
}

func seedFixture(t *testing.T, fx bookingFixture) {
	t.Helper()
	ctx := context.Background()
	routeID, busID, otherBusID := uuid.New(), uuid.New(), uuid.New()
	for _, userID := range []uuid.UUID{fx.user1, fx.user2} {
		if _, err := fx.pool.Exec(ctx, "insert into "+fx.table("users")+" (id) values ($1)", userID); err != nil {
			t.Fatal(err)
		}
	}
	for _, stopID := range []uuid.UUID{fx.a, fx.b, fx.c, fx.d, fx.outside} {
		if _, err := fx.pool.Exec(ctx, "insert into "+fx.table("stops")+" (id) values ($1)", stopID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := fx.pool.Exec(ctx, "insert into "+fx.table("routes")+" (id) values ($1)", routeID); err != nil {
		t.Fatal(err)
	}
	for index, stopID := range []uuid.UUID{fx.a, fx.b, fx.c, fx.d} {
		if _, err := fx.pool.Exec(ctx, "insert into "+fx.table("route_stops")+" (id, route_id, stop_id, stop_order) values ($1,$2,$3,$4)", uuid.New(), routeID, stopID, index+1); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := fx.pool.Exec(ctx, "insert into "+fx.table("buses")+" (id) values ($1),($2)", busID, otherBusID); err != nil {
		t.Fatal(err)
	}
	if _, err := fx.pool.Exec(ctx, "insert into "+fx.table("seats")+" (id, bus_id) values ($1,$2),($3,$4)", fx.seat, busID, fx.otherSeat, otherBusID); err != nil {
		t.Fatal(err)
	}
	if _, err := fx.pool.Exec(ctx, "insert into "+fx.table("trips")+" (id, route_id, bus_id) values ($1,$2,$3)", fx.trip, routeID, busID); err != nil {
		t.Fatal(err)
	}
}

func clearBookings(t *testing.T, fx bookingFixture) {
	t.Helper()
	if _, err := fx.pool.Exec(context.Background(), "truncate "+fx.table("booking_idempotency")+", "+fx.table("bookings")); err != nil {
		t.Fatal(err)
	}
}

func insertExistingBooking(t *testing.T, fx bookingFixture, fromStopID, toStopID uuid.UUID) {
	t.Helper()
	if _, err := fx.pool.Exec(context.Background(), fmt.Sprintf(`
		insert into %s (id, user_id, trip_id, seat_id, from_stop_id, to_stop_id, status, created_at)
		values ($1,$2,$3,$4,$5,$6,'pending',now())
	`, fx.table("bookings")), uuid.New(), fx.user1, fx.trip, fx.seat, fromStopID, toStopID); err != nil {
		t.Fatal(err)
	}
}

func assertBookingCount(t *testing.T, fx bookingFixture, want int) {
	t.Helper()
	var got int
	if err := fx.pool.QueryRow(context.Background(), "select count(*) from "+fx.table("bookings")).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("expected %d bookings, got %d", want, got)
	}
}

var testSchemaStatements = []string{
	`create table {schema}.users (id uuid primary key)`,
	`create table {schema}.stops (id uuid primary key)`,
	`create table {schema}.routes (id uuid primary key)`,
	`create table {schema}.route_stops (id uuid primary key, route_id uuid not null references {schema}.routes(id), stop_id uuid not null references {schema}.stops(id), stop_order integer not null, unique(route_id, stop_id), unique(route_id, stop_order))`,
	`create table {schema}.buses (id uuid primary key)`,
	`create table {schema}.seats (id uuid primary key, bus_id uuid not null references {schema}.buses(id))`,
	`create table {schema}.trips (id uuid primary key, route_id uuid not null references {schema}.routes(id), bus_id uuid not null references {schema}.buses(id))`,
	`create table {schema}.bookings (id uuid primary key, user_id uuid not null references {schema}.users(id), trip_id uuid not null references {schema}.trips(id), seat_id uuid not null references {schema}.seats(id), from_stop_id uuid not null references {schema}.stops(id), to_stop_id uuid not null references {schema}.stops(id), status text not null, created_at timestamptz not null)`,
	`create table {schema}.booking_idempotency (user_id uuid not null references {schema}.users(id), idempotency_key text not null, request_hash bytea not null, booking_id uuid not null unique references {schema}.bookings(id), created_at timestamptz not null default now(), primary key(user_id, idempotency_key))`,
}
