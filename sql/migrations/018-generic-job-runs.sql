-- add scheduling columns directly to urls
alter table urls add column next_check_at timestamptz not null default now();
alter table urls add column last_checked_at timestamptz;
alter table urls add column check_count int not null default 0;
alter table urls add column empty_count int not null default 0;

-- migrate data from url_discussion_jobs
update urls u
set    next_check_at   = j.scheduled_at,
       last_checked_at = j.last_checked_at,
       check_count     = j.check_count,
       empty_count     = j.empty_count
from   url_discussion_jobs j
where  u.id = j.url_id;

-- add scheduling columns directly to feeds
alter table feeds add column next_fetch_at timestamptz not null default now();
alter table feeds add column fetch_count int not null default 0;
alter table feeds add column error_count int not null default 0;
alter table feeds add column last_error text not null default '';

-- migrate data from feed_poll_jobs
update feeds f
set    next_fetch_at = j.scheduled_at,
       fetch_count   = j.fetch_count,
       error_count   = j.error_count,
       last_error    = j.last_error
from   feed_poll_jobs j
where  f.id = j.feed_id;

-- drop old per-item job tables
drop table url_discussion_jobs;
drop table feed_poll_jobs;

-- create generic job tracking tables
create table job_configs (
    job_name     text primary key,
    next_run_at  timestamptz not null default now(),
    locked_until timestamptz not null default '1970-01-01'
);

create table job_runs (
    id           uuid primary key default gen_random_uuid(),
    job_name     text not null references job_configs(job_name) on delete cascade,
    started_at   timestamptz not null default now(),
    completed_at timestamptz,
    error        text,
    metadata     jsonb not null default '{}'::jsonb
);

-- populate job_configs
insert into job_configs (job_name) values ('discuss'), ('feed_poll');

-- migrate next scheduled run for discuss
update job_configs
set    next_run_at = (select scheduled_at from discussion_runs where id = 1)
where  job_name = 'discuss';

-- migrate discussion run history
insert into job_runs (id, job_name, started_at, completed_at, metadata)
select id, 'discuss', started_at, completed_at, jsonb_build_object('url_count', url_count, 'found_count', found_count)
from   discussion_run_log;

-- drop old discussion tracking tables
drop table discussion_run_log;
drop table discussion_runs;
