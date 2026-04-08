create table categories (
	id         uuid        primary key default gen_random_uuid(),
	user_id    uuid        not null references users(id) on delete cascade,
	name       text        not null,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	unique (user_id, name)
);

create table category_members (
	id          uuid primary key default gen_random_uuid(),
	category_id uuid not null references categories(id) on delete cascade,
	kind        text not null check (kind in ('tag', 'domain')),
	value       text not null,
	unique (category_id, kind, value)
);
