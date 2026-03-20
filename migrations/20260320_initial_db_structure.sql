-- 🔥 DROP ALL TABLES (in correct order FK constraints)
drop table if exists public.bookings cascade;
drop table if exists public.trips cascade;
drop table if exists public.route_stops cascade;
drop table if exists public.stops cascade;
drop table if exists public.routes cascade;
drop table if exists public.seats cascade;
drop table if exists public.buses cascade;
drop table if exists public.users cascade;

-- Enable UUID
create extension if not exists "uuid-ossp";

--------------------------------------------------
-- 🟦 USERS (linked to Supabase Auth)
--------------------------------------------------
create table public.users (
  id uuid primary key references auth.users(id) on delete cascade,
  name text,
  email text unique,
  created_at timestamp default now()
);

--------------------------------------------------
-- 🟦 STOPS (normalized locations)
--------------------------------------------------
create table public.stops (
  id uuid primary key default uuid_generate_v4(),
  name text not null,
  city text,
  created_at timestamp default now(),
  unique(name, city)
);

--------------------------------------------------
-- 🟦 ROUTES
--------------------------------------------------
create table public.routes (
  id uuid primary key default uuid_generate_v4(),
  name text,
  created_at timestamp default now()
);

--------------------------------------------------
-- 🟦 ROUTE STOPS (ordered stops per route)
--------------------------------------------------
create table public.route_stops (
  id uuid primary key default uuid_generate_v4(),
  route_id uuid references public.routes(id) on delete cascade,
  stop_id uuid references public.stops(id) on delete cascade,
  stop_order integer not null,
  created_at timestamp default now(),
  unique(route_id, stop_order)
);

--------------------------------------------------
-- 🟦 BUSES
--------------------------------------------------
create table public.buses (
  id uuid primary key default uuid_generate_v4(),
  name text not null,
  total_seats integer not null,
  created_at timestamp default now()
);

--------------------------------------------------
-- 🟦 SEATS
--------------------------------------------------
create table public.seats (
  id uuid primary key default uuid_generate_v4(),
  bus_id uuid references public.buses(id) on delete cascade,
  seat_number text not null,
  created_at timestamp default now(),
  unique(bus_id, seat_number)
);

--------------------------------------------------
-- 🟨 TRIPS (scheduled bus runs)
--------------------------------------------------
create table public.trips (
  id uuid primary key default uuid_generate_v4(),
  route_id uuid references public.routes(id) on delete cascade,
  bus_id uuid references public.buses(id) on delete cascade,
  departure_time timestamp not null,
  arrival_time timestamp not null,
  price numeric(10,2) not null,
  created_at timestamp default now()
);

--------------------------------------------------
-- 🟨 BOOKINGS
--------------------------------------------------
create table public.bookings (
  id uuid primary key default uuid_generate_v4(),
  user_id uuid references public.users(id) on delete cascade,
  trip_id uuid references public.trips(id) on delete cascade,
  seat_id uuid references public.seats(id) on delete set null,
  from_stop_id uuid references public.route_stops(id),
  to_stop_id uuid references public.route_stops(id),
  status text check (status in ('confirmed', 'cancelled')) default 'confirmed',
  created_at timestamp default now()
);

--------------------------------------------------
-- ⚡ INDEXES (performance)
--------------------------------------------------
create index idx_route_stops_route on public.route_stops(route_id);
create index idx_trips_route on public.trips(route_id);
create index idx_bookings_trip on public.bookings(trip_id);
create index idx_bookings_user on public.bookings(user_id);