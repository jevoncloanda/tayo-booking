--------------------------------------------------
-- 🔐 REFRESH TOKENS (session management)
--------------------------------------------------
create table public.refresh_tokens (
  id uuid primary key default uuid_generate_v4(),

  user_id uuid not null 
    references public.users(id) on delete cascade,

  token text not null unique,

  expires_at timestamp not null,
  created_at timestamp default now(),

  revoked boolean default false,

  user_agent text,
  ip_address text
);

--------------------------------------------------
-- ⚡ INDEXES
--------------------------------------------------
create index idx_refresh_tokens_user on public.refresh_tokens(user_id);
create index idx_refresh_tokens_token on public.refresh_tokens(token);