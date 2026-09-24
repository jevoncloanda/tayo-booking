# PROJECT_CONTEXT.md

> **For AI Assistants**
>
> Read this document before making any code changes or architectural decisions.
> Treat the PostgreSQL schema as the authoritative source for database definitions and relationships.
> If information is missing, state your assumptions — do not invent features or modify the architecture.
> Always read the existing codebase files before writing any new code. Match existing patterns exactly.

---

## Project Overview

TAYO is a modern bus ticket booking platform where passengers can browse scheduled trips, choose boarding and alighting stops, reserve a seat, pay for their ticket, and manage their bookings.

The defining feature of TAYO is **segment-based seat occupancy**: the same seat can be reused by multiple passengers on non-overlapping segments of the same trip. This must be preserved in all implementations.

The long-term vision includes an official **Ticket Marketplace** for passengers to securely resell confirmed tickets through the platform. This feature is explicitly **deferred** and must not be implemented until instructed.

---

## Project Goals

- Clean Architecture
- Scalability & Maintainability
- Security
- Good user experience
- Accurate real-world bus operations modeling
- Normalized relational database design

---

## Tech Stack

- **Language:** Go
- **Framework:** Gin
- **Database:** PostgreSQL via Supabase
- **Driver:** pgx/v5
- **Auth:** JWT Access Tokens + Refresh Tokens via HTTP-only cookies
- **Frontend:** Next.js (to be integrated later — backend must be fully tested first)

---

## Core Domain

### Stops

A physical location where passengers board or leave the bus. Stops are reusable across multiple routes.

### Routes

An ordered path through multiple stops. Routes are **directional templates** — they contain no scheduling information. Jakarta → Bandung and Bandung → Jakarta are **two separate routes**.

### Route Stops

The ordered sequence of stops within a route. `stop_order` determines travel direction and is essential for segment-based seat calculations. A stop cannot appear twice on the same route.

### Buses

A physical vehicle with a fixed seat layout defined by:
- `columns_left` — number of seat columns left of the aisle
- `columns_right` — number of seat columns right of the aisle

`total_columns` is never stored — it is always derived as `columns_left + columns_right`.

Different bus types can have different layouts:
```
Standard:   columns_left=2, columns_right=2  →  2 | aisle | 2
Executive:  columns_left=1, columns_right=2  →  1 | aisle | 2
VIP:        columns_left=1, columns_right=1  →  1 | aisle | 1
```

### Seats

Seats belong to a bus and are reused across trips (same bus, multiple departures). Each seat has:
- `seat_number` — e.g. "A1", "A2"
- `seat_row` — e.g. "A", "B", "C"
- `seat_column` — e.g. 1, 2, 3, 4

Seats are **auto-generated** when a bus is created via `POST /admin/buses`. Admin can manually edit individual seat numbers via `PATCH /admin/seats/:id`.

The aisle position is determined at render time by the frontend using `columns_left` from the bus object — it is never stored in the seat record.

### Trips

A scheduled execution of a route. Combines:
- One Route
- One Bus
- Departure Time / Arrival Time
- Ticket Price
- `max_cancellation_minutes` — how many minutes before departure a user may cancel

Passengers always book **Trips**, never Routes directly.

### Bookings

One passenger reserving one seat on one trip. Records:
- Passenger (user)
- Trip
- Seat
- Boarding Stop (`from_stop_id`)
- Destination Stop (`to_stop_id`)
- Status (`pending` / `confirmed` / `cancelled`)

---

## Booking Flow

```
Browse Available Trips
  ▼
Select Trip
  ▼
View Route & Stops
  ▼
Choose Boarding Stop → Choose Destination Stop
  ▼
View Available Seats (segment-aware)
  ▼
Select Seat
  ▼
Review Booking
  ▼
POST /bookings → status: pending
  ▼
Admin confirms via PATCH /admin/bookings/:id → status: confirmed
  ▼
(Future) Payment integration will automate confirmation
```

Bookings are created with `pending` status. An admin manually confirms them via `PATCH /admin/bookings/:id`. When payment is integrated later, confirmation will happen automatically on payment success.

---

## Booking Status Lifecycle

| Status | Meaning |
|---|---|
| `pending` | Seat reserved, awaiting admin confirmation (or future payment) |
| `confirmed` | Confirmed by admin (or future payment success) |
| `cancelled` | Cancelled by user or admin |

### Status Transition Rules

- Users may cancel `pending` bookings only
- Users **cannot** cancel `confirmed` bookings
- Cancellation is only allowed if `now < departure_time - max_cancellation_minutes`
- If cancellation window has passed, return `422` with message `"Cancellation window has passed"`
- Admin may set status to `confirmed` or `cancelled` on any booking
- Cancelled bookings are **never deleted** — status is set to `cancelled` and the row is kept for audit

---

## Segment-Based Seat Occupancy

> This is the most critical business rule in TAYO. Never simplify it.

Seats are reserved only for a passenger's travel **segment**, not the entire trip. This allows seat reuse on non-overlapping segments.

Example — Route: A → B → C → D → E

- Passenger 1 books Seat 5: A → C (occupies A–B and B–C)
- Passenger 2 books Seat 5: C → E ✅ valid (no overlap)
- Passenger 3 tries Seat 5: B → D ❌ invalid (overlaps with Passenger 1)

### Overlap Condition (SQL)

Two segments overlap when:
```sql
existing.from_order < requested.to_order
AND existing.to_order > requested.from_order
```

This condition must be used in every seat availability query. Use CTEs to compute `stop_order` values from `route_stops` before applying this condition.

### Availability Check on Booking

When `POST /bookings` is called:
1. Validate `from_stop_id` and `to_stop_id` belong to the trip's route
2. Validate `from_stop_id` comes before `to_stop_id` in `stop_order`
3. Validate the seat belongs to the trip's bus
4. Run the segment overlap query against all `confirmed` bookings on the same trip
5. If the seat is unavailable → `409 Conflict`
6. Run the availability check and insert in a **single transaction** to prevent race conditions

---

## Authentication

- **JWT Access Token** — short-lived (1 hour), contains `user_id` and `role`, never stored in DB
- **Refresh Token** — long-lived (7 days), stored in `refresh_tokens` table, rotated on every refresh
- Both delivered via **HTTP-only cookies**
- `auth.users` stores credentials (Supabase); `public.users` stores profile data

### Role System

- `users.role` column: `'user'` or `'admin'`
- Role is embedded in the JWT at login time so middleware never needs a DB round-trip
- On token refresh, role is re-fetched from DB so role changes take effect on next refresh without forced re-login
- `AuthMiddleware` injects `user_id` and `role` into Gin context
- `AdminMiddleware` reads `role` from Gin context (never re-validates JWT)

### Refresh Token Rotation

Every call to `POST /auth/refresh`:
- Invalidates the old refresh token
- Issues a new refresh token
- Issues a new access token with current role from DB

This means stolen tokens become invalid as soon as the real user refreshes. If a stolen token is used first, the real user's next refresh will fail — signalling a potential compromise.

### Refresh Token Cleanup

The `refresh_tokens` table grows over time. Run a periodic cleanup:
```sql
delete from refresh_tokens
where expires_at < now()
   or revoked = true;
```
Recommended: Supabase `pg_cron` scheduled daily at low-traffic hours.

---

## Authorization

- Users may only access and modify their own resources
- `user_id` is always read from Gin context (set by `AuthMiddleware`) — never from the request body
- Authorization is always enforced on the **backend**

---

## Backend Architecture

```
Handler
  ▼
Service
  ▼
Repository
  ▼
PostgreSQL
```

### Handler
- Parses HTTP requests
- Validates request format and required fields (UUID format, required fields, time formats)
- Calls the appropriate service
- Formats and returns HTTP responses
- **No business logic**

### Service
- All business logic lives here
- Booking validation and rules
- Seat availability calculations (segment-aware)
- Authorization checks (ownership)
- Orchestrates repository calls
- **No Gin, no HTTP, no cookies**

### Repository
- SQL queries only
- Database mapping
- Transactions for multi-table operations
- **No business logic**

---

## Code Style & Patterns

### Naming
- Constructor pattern: `NewXxx()`
- Dependency injection throughout
- Early returns, explicit error handling

### SQL
- Always **lowercase** SQL keywords
- Use CTEs for complex availability queries
- Always use parameterized queries — never string concatenation

### Duplicate Detection
- Use query-first pattern (fetch before insert) — **do not use `pgconn` directly**
- Consistent with how auth checks duplicate emails
- Example: `GetStopByNameAndCity` before `CreateStop`

### Nil Slice Normalisation
- Always return `[]` not `null` for list endpoints:
```go
if result == nil {
    result = []models.Xxx{}
}
```

### Error Handling
- Never expose raw database errors to clients
- Map to clean domain errors in the service layer
- Return structured errors from handlers:
```json
{ "error": "Booking not found" }
```

### Transactions
Use pgx transactions for any operation touching multiple tables:
- `POST /admin/buses` — insert bus + all seats atomically
- `POST /bookings` — availability check + insert atomically

---

## Database Schema

### users
| Column | Type | Notes |
|--------|------|-------|
| id | UUID | references auth.users |
| name | TEXT | |
| email | TEXT | unique |
| role | TEXT | `'user'` or `'admin'`, default `'user'` |
| created_at | TIMESTAMP | |

### stops
| Column | Type | Notes |
|--------|------|-------|
| id | UUID | |
| name | TEXT | unique with city |
| city | TEXT | |
| created_at | TIMESTAMP | |

### routes
| Column | Type |
|--------|------|
| id | UUID |
| name | TEXT |
| created_at | TIMESTAMP |

### route_stops
| Column | Type | Notes |
|--------|------|-------|
| id | UUID | |
| route_id | UUID | |
| stop_id | UUID | |
| stop_order | INTEGER | 1-based, unique per route |
| created_at | TIMESTAMP | |

Constraints: `UNIQUE(route_id, stop_order)`, `UNIQUE(route_id, stop_id)`

### buses
| Column | Type | Notes |
|--------|------|-------|
| id | UUID | |
| name | TEXT | |
| total_seats | INTEGER | computed at creation: `total_rows * (columns_left + columns_right)` |
| columns_left | INTEGER | seats left of aisle |
| columns_right | INTEGER | seats right of aisle |
| created_at | TIMESTAMP | |

### seats
| Column | Type | Notes |
|--------|------|-------|
| id | UUID | |
| bus_id | UUID | |
| seat_number | TEXT | e.g. "A1" |
| seat_row | TEXT | e.g. "A" |
| seat_column | INTEGER | e.g. 1, 2, 3, 4 |
| created_at | TIMESTAMP | |

Constraint: `UNIQUE(bus_id, seat_number)`

### trips
| Column | Type | Notes |
|--------|------|-------|
| id | UUID | |
| route_id | UUID | |
| bus_id | UUID | |
| departure_time | TIMESTAMP | |
| arrival_time | TIMESTAMP | |
| price | NUMERIC(10,2) | |
| max_cancellation_minutes | INTEGER | NOT NULL DEFAULT 0 |
| created_at | TIMESTAMP | |

### bookings
| Column | Type | Notes |
|--------|------|-------|
| id | UUID | |
| user_id | UUID | |
| trip_id | UUID | |
| seat_id | UUID | |
| from_stop_id | UUID | references stops |
| to_stop_id | UUID | references stops |
| status | TEXT | `pending` / `confirmed` / `cancelled` |
| created_at | TIMESTAMP | |

### refresh_tokens
| Column | Type |
|--------|------|
| id | UUID |
| user_id | UUID |
| token | TEXT |
| expires_at | TIMESTAMP |
| revoked | BOOLEAN |
| created_at | TIMESTAMP |
| user_agent | TEXT |
| ip_address | TEXT |

---

## API Reference

### Auth (public)
```
POST /auth/register
POST /auth/login
POST /auth/refresh
POST /auth/logout
GET  /me                          ← AuthMiddleware
```

### Trips (public)
```
GET  /trips                       ← optional: from_stop_id, to_stop_id, date
GET  /trips/:id
GET  /trips/:id/seats             ← required: from_stop_id, to_stop_id
```

### Bookings (authenticated)
```
POST   /bookings
GET    /bookings
GET    /bookings/:id
DELETE /bookings/:id              ← cancellation
```

### Admin (AuthMiddleware + AdminMiddleware)
```
POST  /admin/stops
POST  /admin/routes
POST  /admin/route-stops
POST  /admin/buses
PATCH /admin/seats/:id
POST  /admin/trips
PATCH /admin/trips/:id
GET   /admin/bookings             ← optional: status, trip_id
PATCH /admin/bookings/:id
```

---

## Admin Route Group Pattern

All admin routes are grouped and share stacked middleware:

```go
admin := r.Group("/admin")
admin.Use(handlers.AuthMiddleware(), handlers.AdminMiddleware())
```

Middleware runs in order — `AuthMiddleware` first (injects `user_id` and `role`), then `AdminMiddleware` (reads `role` from context). Never reverse this order.

---

## Seat Map UI (Frontend — planned)

The frontend renders a theater-style seat picker using:
- `columns_left` and `columns_right` from the bus object
- `seat_row` and `seat_column` from each seat
- `status` (`available` / `booked`) from `GET /trips/:id/seats`

The aisle gap is rendered between column `columns_left` and column `columns_left + 1`. The frontend never hardcodes aisle position — it always reads from the bus layout.

---

## Migrations

All migration files use lowercase SQL. Naming convention: `YYYYMMDD_description.sql`.

Do not use `ADD CONSTRAINT IF NOT EXISTS` — it is invalid PostgreSQL syntax. Use `DROP CONSTRAINT IF EXISTS` followed by an unconditional `ADD CONSTRAINT`.

---

## What Is Deferred

- Payment integration
- Ticket Marketplace (resale)
- Frontend (Next.js) — start only after backend is fully tested end to end

---

## End-to-End Test Order (Postman)

Before starting the frontend, verify the full flow:

```
1.  POST /auth/register + POST /auth/login (admin)
2.  POST /admin/stops         ← create 4 stops
3.  POST /admin/routes        ← create a route
4.  POST /admin/buses         ← verify seats are auto-generated
5.  POST /admin/route-stops   ← assign stops in order 1,2,3,4
6.  POST /admin/trips         ← create trip using route + bus IDs
7.  GET  /trips               ← verify trip appears
8.  GET  /trips/:id           ← verify stops are ordered correctly
9.  GET  /trips/:id/seats     ← all seats should be "available"
10. POST /bookings            ← book seat (e.g. stop 1 → stop 3)
11. GET  /trips/:id/seats     ← that seat should be "booked" for overlapping segments
                                 but "available" for non-overlapping (e.g. stop 3 → stop 4)
12. PATCH /admin/bookings/:id ← confirm the booking
13. DELETE /bookings/:id      ← test cancellation rules
```

Step 11 is the most critical — it verifies the segment overlap logic is working correctly.