create table discussion_runs (
	id           int         primary key default 1 check (id = 1),
	scheduled_at timestamptz not null default now(),
	last_run_at  timestamptz
);

insert into discussion_runs (id) values (1);
