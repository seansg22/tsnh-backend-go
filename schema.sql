create table if not exists users (
  username   text primary key,
  code       text not null,
  data       jsonb not null default '{}',
  updated_at timestamptz not null default now()
);
