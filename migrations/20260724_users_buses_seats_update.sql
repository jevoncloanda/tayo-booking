-- Add role to users
alter table public.users
  add column role text not null default 'user'
  check (role in ('user', 'admin'));

-- Bus layout config
alter table public.buses
  add column columns_left  integer not null default 2,
  add column columns_right integer not null default 2;

-- Seat position
alter table public.seats
  add column seat_row    text,
  add column seat_column integer;