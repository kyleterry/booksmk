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
