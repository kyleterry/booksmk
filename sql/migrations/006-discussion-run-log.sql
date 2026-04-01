create table discussion_run_log (
	id           uuid        primary key default gen_random_uuid(),
	started_at   timestamptz not null,
	completed_at timestamptz not null default now(),
	url_count    int         not null default 0,
	found_count  int         not null default 0
);
