# Booking creation correctness

## Segment-aware occupancy

A booking occupies a half-open route interval: `[from_stop_order, to_stop_order)`.
Two bookings overlap when the existing origin is before the requested destination
and the existing destination is after the requested origin. This allows one seat
to serve `A-B` and then `B-D`, while rejecting `A-C` plus `B-D`.

Both `pending` and `confirmed` bookings occupy their segments. Cancelled bookings
do not.

## Original race

Booking creation previously opened a default PostgreSQL transaction, queried for
an overlapping booking, inserted when none existed, and committed. Under the
default `READ COMMITTED` isolation level, two transactions could both observe the
same earlier committed state. No selected row existed to lock, and the schema had
no constraint representing route-interval overlap. Both transactions could
therefore insert an overlapping booking and commit.

Wrapping check and insert in one transaction did not make that pair of statements
atomic relative to another transaction.

## Concurrency boundary

Creation now acquires a transaction-scoped PostgreSQL advisory lock for
`(trip_id, seat_id)` before validating and checking availability. All API
instances use the same database lock, so competing operations for one physical
trip seat serialize. The second transaction checks availability only after the
first commits or rolls back.

The lock serializes operations for a seat only while they execute. It does not
change persisted occupancy: the overlap query remains segment-aware, so adjacent
segments still succeed. Hash collisions can cause unrelated operations to wait,
but cannot allow conflicting operations through.

An exclusion constraint would make the invariant declarative, but the current
schema stores stop IDs rather than a directly constrainable range. Maintaining a
derived range would require wider schema and update rules. The advisory lock is
the smallest robust change for this schema.

## Idempotency

`POST /bookings` requires an `Idempotency-Key` header. Keys are owned by the
authenticated user. The server hashes the logical payload `(trip, seat, origin,
destination)` and persists the hash with the created booking under a unique
`(user_id, idempotency_key)` key.

Transaction order:

1. Begin transaction.
2. Acquire transaction-scoped `(user_id, idempotency_key)` advisory lock.
3. Replay an existing result when its request hash matches; reject mismatches.
4. Acquire transaction-scoped `(trip_id, seat_id)` advisory lock.
5. Validate route stops, direction, and bus-seat ownership.
6. Recheck segment availability.
7. Insert booking and idempotency result.
8. Commit, releasing both locks.

The fixed lock order prevents deadlocks between booking creation paths. Concurrent
retries with one key create one booking; concurrent users with different keys but
one overlapping seat produce one booking and one conflict.

Replays return the original creation representation, including its original
`pending` status, even if later administration changes the live booking status.

Concurrency control answers whether competing operations can violate occupancy.
Idempotency answers whether retrying one logical operation can create it twice.
Both are required.

Only successful results are stored. Validation and seat-conflict failures may be
retried with the same key. Idempotency rows currently have no automatic expiry;
a future maintenance job may delete rows after the business retention window.
No background infrastructure is needed for current volume.

## Integration tests

Repository integration tests require `TEST_DATABASE_URL`. They create and drop a
unique temporary schema, never using application tables. The suite covers segment
boundaries, invalid domain inputs, concurrent overlapping and adjacent bookings,
sequential replay, key misuse, and concurrent same-key retries.
