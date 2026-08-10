# PROJECT_CONTEXT.md

> **For AI Assistants**
>
> Read this document before making any code changes or architectural decisions.
> Treat the PostgreSQL schema as the authoritative source for database definitions and relationships.
> If information is missing, state your assumptions — do not invent features or modify the architecture.

---

## Project Overview

TAYO is a modern bus ticket booking platform where passengers can browse scheduled trips, choose boarding and alighting stops, reserve a seat, pay for their ticket, and manage their bookings.

The defining feature of TAYO is **segment-based seat occupancy**: the same seat can be reused by multiple passengers on non-overlapping segments of the same trip. This must be preserved in all implementations.

The long-term vision includes an official **Ticket Marketplace** for passengers to securely resell confirmed tickets through the platform.

---

## Project Goals

- Clean Architecture
- Scalability & Maintainability
- Security
- Good user experience
- Accurate real-world bus operations modeling
- Normalized relational database design

---

## Core Domain

### Stops

A physical location where passengers board or leave the bus. Stops are reusable across multiple routes.

### Routes

An ordered path through multiple stops. Routes are **templates** — they contain no scheduling information.

Example:
```
Jakarta → Bekasi → Karawang → Bandung
```

### Route Stops

The ordered sequence of stops within a route. Ordering determines travel direction and is essential for segment-based seat calculations.

### Buses

A physical vehicle with a fixed collection of seats.

### Seats

Seats belong to a bus and are reused across trips (same bus, multiple departures).

### Trips

A scheduled execution of a route. Combines:
- One Route
- One Bus
- Departure Time / Arrival Time
- Ticket Price

Passengers always book **Trips**, never Routes directly.

### Bookings

One passenger reserving one seat on one trip. Records:
- Passenger (user)
- Trip
- Seat
- Boarding Stop (`from_stop_id`)
- Destination Stop (`to_stop_id`)
- Status

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
Create Pending Booking (seat temporarily reserved)
  ▼
Complete Payment
  ▼
  ├── Success → Confirmed
  └── Timeout / Failure → Expired / Cancelled
```

Bookings are **not confirmed until payment succeeds**. Pending bookings hold the seat temporarily; if payment doesn't complete within the allowed window, the booking expires and the seat is released.

---

## Booking Status Lifecycle

| Status | Meaning |
|---|---|
| `pending` | Seat reserved, awaiting payment |
| `confirmed` | Payment successful |
| `cancelled` | Cancelled by user or system |
| `expired` | Payment window elapsed |

Status transitions must be validated in the service layer.

---

## Segment-Based Seat Occupancy

> This is the most critical business rule in TAYO. Never simplify it.

Seats are reserved only for a passenger's travel **segment**, not the entire trip. This allows seat reuse on non-overlapping segments.

Example — Route: A → B → C → D → E

- Passenger 1 books Seat 5: A → C (occupies A–B and B–C)
- Passenger 2 books Seat 5: C → E ✅ valid (no overlap)
- Passenger 3 tries Seat 5: B → D ❌ invalid (overlaps with Passenger 1's segment)

Every seat availability query must evaluate segment overlap against `route_stops.stop_order`. Never treat seats as simply "booked" or "available" for a whole trip.

---

## Ticket Marketplace (Planned Feature)

Allows passengers to resell confirmed tickets securely through TAYO. Prevents scams and scalping by acting as the trusted intermediary.

### Core Principles

- Passengers may list confirmed bookings for resale
- On successful sale, **booking ownership transfers** to the buyer (no duplicate bookings created)
- TAYO handles all payments — no direct user-to-user transactions

### Pricing Rules

- Seller sets their own price, subject to: `selling_price ≤ original_purchase_price`
- Discounts are allowed; price increases (scalping) are not

### Marketplace Flow

```
Seller Lists Ticket → Ticket Locked
  ▼
Buyer Pays TAYO
  ▼
Payment Verified
  ▼
Booking Ownership Transfers to Buyer
  ▼
Seller Receives Payment from TAYO
```

---

## Authentication

- **JWT Access Tokens** — short-lived (1 hour), authenticate API requests
- **Refresh Tokens** — long-lived (7 days), manage sessions; support rotation, revocation, and logout
- Tokens are delivered via **HTTP-only cookies**
- Application user data (`public.users`) is separate from auth credentials (`auth.users`)

---

## Authorization

- Users may only access and modify their own resources
- Authorization is always enforced on the **backend** — never rely on frontend validation

---

## Backend Architecture

```
Handler (Controller)
  ▼
Service
  ▼
Repository
  ▼
PostgreSQL
```

### Handler

- Parses HTTP requests
- Validates request format and required fields
- Calls the appropriate service
- Formats and returns HTTP responses

Handlers must remain thin — no business logic.

### Service

Contains all business logic:
- Booking validation and rules
- Seat availability calculations (segment-aware)
- Payment processing
- Marketplace logic
- Authorization checks
- Database transactions

### Repository

Interacts with PostgreSQL only:
- CRUD operations
- SQL queries
- Database mapping

No business logic in repositories.

---

## Validation Rules

Always validate:
- Authentication (valid JWT)
- Authorization (resource ownership)
- UUIDs (valid format, entity exists)
- Required fields
- Stop ordering (segment direction)
- Seat availability (segment-aware)
- Trip existence
- Booking ownership before mutations
- Payment status before confirming bookings

Never trust client input.

---

## API Design

Follows REST conventions.

```
POST   /auth/register
POST   /auth/login
POST   /auth/logout
POST   /auth/refresh

GET    /users/me

GET    /trips
GET    /trips/:id

POST   /bookings
GET    /bookings
DELETE /bookings/:id
```

Responses must be consistent throughout the API. Return appropriate HTTP status codes.

---

## Security Principles

Always:
- Verify JWTs on every protected request
- Validate Refresh Tokens (not revoked, not expired)
- Use parameterized SQL (no string concatenation)
- Validate ownership before any resource mutation
- Handle errors safely without leaking internals

Never:
- Trust frontend validation
- Construct SQL strings manually
- Expose sensitive implementation details in API responses

---

## Database Philosophy

The schema is intentionally normalized. Prefer relationships over duplication:
- Stops are reusable across routes
- Routes are reused by trips
- Buses are reused across trips
- Seats belong to buses (not trips)
- Bookings reference seats

Avoid redundant data storage.

---

## Development Principles

When extending TAYO:
- Preserve the layered architecture strictly
- Keep handlers thin
- Place all business logic in services
- Keep repositories focused on DB access only
- Reuse existing entities wherever possible
- Prefer maintainable solutions over shortcuts
- Architecture consistency > feature velocity

---

## Long-Term Vision

TAYO aims to be a modern, trustworthy bus booking platform with:
- Accurate segment-based seat management
- Secure authentication and session management
- Reliable booking lifecycle management
- Official ticket resale marketplace
- Fair pricing enforcement
- Scam-resistant ownership transfers
- Clean, scalable software architecture

Every new feature should align with these principles and preserve the integrity of the overall system.