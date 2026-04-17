-- name: CreateJobRun :one
insert into job_runs (job_name, started_at)
values ($1, $2)
returning id;

-- name: CompleteJobRun :exec
update job_runs
set completed_at = $2,
    error        = $3,
    metadata     = $4
where id = $1;

-- name: ListJobRuns :many
select id, job_name, started_at, completed_at, error, metadata
from job_runs
where job_name = $1
order by started_at desc
limit $2;

-- name: GetJobConfig :one
select job_name, next_run_at, locked_until
from job_configs
where job_name = $1;

-- name: ClaimJobRun :one
update job_configs
set    locked_until = $2,
       next_run_at  = $3
where  job_name = $1
  and  next_run_at <= now()
  and  locked_until <= now()
returning job_name;

-- name: UpdateJobNextRun :exec
update job_configs
set next_run_at = $2
where job_name = $1;
