create table users (
	id              uuid         primary key default gen_random_uuid(),
	email           text         not null unique,
	password_digest text         not null,
	is_admin        boolean      not null default false,
	theme           text         not null default 'dark' check (theme in ('dark', 'light', 'auto')),
	font_size       text         not null default 'medium' check (font_size in ('small', 'medium', 'large')),
	created_at      timestamptz  not null default now(),
	updated_at      timestamptz  not null default now()
);

create table sessions (
	token      text        primary key,
	user_id    uuid        not null references users(id) on delete cascade,
	created_at timestamptz not null default now(),
	expires_at timestamptz not null
);

create table urls (
	id         uuid         primary key default gen_random_uuid(),
	url        text         not null unique,
	feed_url   text         not null default '',
	created_at timestamptz  not null default now()
);

create table user_urls (
	user_id     uuid         not null references users(id) on delete cascade,
	url_id      uuid         not null references urls(id) on delete cascade,
	title       text         not null default '',
	description text         not null default '',
	created_at  timestamptz  not null default now(),
	updated_at  timestamptz  not null default now(),
	primary key (user_id, url_id)
);

create table tags (
	id   uuid   primary key default gen_random_uuid(),
	name text   not null unique
);

create table url_tags (
	user_id uuid not null references users(id) on delete cascade,
	url_id  uuid not null references urls(id) on delete cascade,
	tag_id  uuid not null references tags(id) on delete cascade,
	primary key (user_id, url_id, tag_id)
);

create table invite_codes (
	id         uuid        primary key default gen_random_uuid(),
	code       text        not null unique,
	created_by uuid        not null references users(id) on delete cascade,
	used_by    uuid        references users(id) on delete set null,
	used_at    timestamptz,
	created_at timestamptz not null default now()
);

create table api_keys (
	id           uuid        primary key default gen_random_uuid(),
	user_id      uuid        not null references users(id) on delete cascade,
	name         text        not null,
	token_hash   text        not null unique,
	token_prefix text        not null,
	expires_at   timestamptz,
	created_at   timestamptz not null default now()
);

create table url_discussions (
	id             uuid        primary key default gen_random_uuid(),
	url_id         uuid        not null references urls(id) on delete cascade,
	source         text        not null,
	title          text        not null,
	discussion_url text        not null,
	score          int         not null default 0,
	comment_count  int         not null default 0,
	found_at       timestamptz not null default now(),
	unique (url_id, discussion_url)
);

create table url_discussion_jobs (
	id              uuid        primary key default gen_random_uuid(),
	url_id          uuid        not null references urls(id) on delete cascade unique,
	scheduled_at    timestamptz not null default now(),
	last_checked_at timestamptz,
	check_count     int         not null default 0,
	empty_count     int         not null default 0
);

create table discussion_runs (
	id           int         primary key default 1 check (id = 1),
	scheduled_at timestamptz not null default now(),
	last_run_at  timestamptz
);

insert into discussion_runs (id) values (1);

create table discussion_run_log (
	id           uuid        primary key default gen_random_uuid(),
	started_at   timestamptz not null,
	completed_at timestamptz not null default now(),
	url_count    int         not null default 0,
	found_count  int         not null default 0
);

create table feeds (
	id              uuid        primary key default gen_random_uuid(),
	feed_url        text        not null unique,
	site_url        text        not null default '',
	title           text        not null default '',
	description     text        not null default '',
	image_url       text        not null default '',
	last_fetched_at timestamptz,
	created_at      timestamptz not null default now(),
	updated_at      timestamptz not null default now()
);

create table user_feeds (
	user_id     uuid        not null references users(id) on delete cascade,
	feed_id     uuid        not null references feeds(id) on delete cascade,
	custom_name text        not null default '',
	created_at  timestamptz not null default now(),
	primary key (user_id, feed_id)
);

create table feed_tags (
	user_id uuid not null references users(id) on delete cascade,
	feed_id uuid not null references feeds(id) on delete cascade,
	tag_id  uuid not null references tags(id) on delete cascade,
	primary key (user_id, feed_id, tag_id)
);

create table feed_items (
	id           uuid        primary key default gen_random_uuid(),
	feed_id      uuid        not null references feeds(id) on delete cascade,
	guid         text        not null,
	url          text        not null default '',
	title        text        not null default '',
	summary      text        not null default '',
	author       text        not null default '',
	published_at timestamptz,
	created_at   timestamptz not null default now(),
	unique (feed_id, guid)
);

create table feed_item_reads (
	user_id uuid        not null references users(id) on delete cascade,
	item_id uuid        not null references feed_items(id) on delete cascade,
	read_at timestamptz not null default now(),
	primary key (user_id, item_id)
);

create table feed_poll_jobs (
	id              uuid        primary key default gen_random_uuid(),
	feed_id         uuid        not null references feeds(id) on delete cascade unique,
	scheduled_at    timestamptz not null default now(),
	last_fetched_at timestamptz,
	fetch_count     int         not null default 0,
	error_count     int         not null default 0,
	last_error      text        not null default ''
);

create index on feed_items (feed_id, published_at desc);
create index on feed_poll_jobs (scheduled_at);
