create table blocklist (
	id         uuid        primary key default gen_random_uuid(),
	pattern    text        not null unique,
	kind       text        not null check (kind in ('domain', 'url')),
	created_at timestamptz not null default now()
);

alter table urls add column is_blocked_bypass boolean not null default false;
alter table feeds add column is_blocked_bypass boolean not null default false;
