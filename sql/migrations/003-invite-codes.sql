alter table users add column is_admin boolean not null default false;

create table invite_codes (
	id         uuid        primary key default gen_random_uuid(),
	code       text        not null unique,
	created_by uuid        not null references users(id) on delete cascade,
	used_by    uuid        references users(id) on delete set null,
	used_at    timestamptz,
	created_at timestamptz not null default now()
);
