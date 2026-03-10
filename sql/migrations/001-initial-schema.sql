create table users (
	id              uuid        primary key default gen_random_uuid(),
	email           text        not null unique,
	password_digest text        not null,
	created_at      timestamptz not null default now(),
	updated_at      timestamptz not null default now()
);

create table sessions (
	token      text        primary key,
	user_id    uuid        not null references users(id) on delete cascade,
	created_at timestamptz not null default now(),
	expires_at timestamptz not null
);

create table urls (
	id         uuid        primary key default gen_random_uuid(),
	url        text        not null unique,
	created_at timestamptz not null default now()
);

create table user_urls (
	user_id     uuid        not null references users(id) on delete cascade,
	url_id      uuid        not null references urls(id) on delete cascade,
	title       text        not null default '',
	description text        not null default '',
	created_at  timestamptz not null default now(),
	updated_at  timestamptz not null default now(),
	primary key (user_id, url_id)
);

create table tags (
	id   uuid primary key default gen_random_uuid(),
	name text not null unique
);

create table url_tags (
	user_id uuid not null references users(id) on delete cascade,
	url_id  uuid not null references urls(id) on delete cascade,
	tag_id  uuid not null references tags(id) on delete cascade,
	primary key (user_id, url_id, tag_id)
);
