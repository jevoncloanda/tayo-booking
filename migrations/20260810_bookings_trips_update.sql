-- fix booking stop references: point to stops table instead of route_stops
alter table public.bookings
  drop constraint if exists bookings_from_stop_id_fkey,
  drop constraint if exists bookings_to_stop_id_fkey;

alter table public.bookings
  add constraint bookings_from_stop_id_fkey
    foreign key (from_stop_id) references public.stops(id),
  add constraint bookings_to_stop_id_fkey
    foreign key (to_stop_id) references public.stops(id);

-- add pending to allowed statuses and change default
alter table public.bookings
  drop constraint if exists bookings_status_check;

alter table public.bookings
  add constraint bookings_status_check
    check (status in ('pending', 'confirmed', 'cancelled')),
  alter column status set default 'pending';

-- add max_cancellation_minutes to trips
alter table public.trips
  add column if not exists max_cancellation_minutes integer not null default 0;

-- add unique constraint for (route_id, stop_id) on route_stops to prevent same stop twice on a route
alter table public.route_stops
  drop constraint if exists route_stops_route_stop_unique;

alter table public.route_stops
  add constraint route_stops_route_stop_unique unique (route_id, stop_id);
