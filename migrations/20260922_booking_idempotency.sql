-- Booking creation idempotency. Concurrency safety itself uses transaction-scoped
-- PostgreSQL advisory locks and therefore needs no persisted lock rows.
create table public.booking_idempotency (
  user_id uuid not null references public.users(id) on delete cascade,
  idempotency_key text not null,
  request_hash bytea not null,
  booking_id uuid not null references public.bookings(id) on delete cascade,
  created_at timestamptz not null default now(),
  primary key (user_id, idempotency_key),
  unique (booking_id),
  constraint booking_idempotency_key_length
    check (char_length(idempotency_key) between 1 and 255)
);

-- Rollback, if required before dependent schema changes:
-- drop table public.booking_idempotency;
