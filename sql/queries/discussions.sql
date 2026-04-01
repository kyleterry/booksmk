-- name: EnqueueDiscussionJob :exec
insert into url_discussion_jobs (url_id)
values ($1)
on conflict (url_id) do nothing;

-- name: ClaimBatchRun :one
update discussion_runs
set    scheduled_at = now() + interval '30 minutes',
       last_run_at  = now()
where  scheduled_at <= now()
returning id;

-- name: ListDueURLs :many
select j.id, j.url_id, u.url, j.check_count, j.empty_count
from url_discussion_jobs j
join urls u on u.id = j.url_id
where j.scheduled_at <= now()
order by j.scheduled_at
limit 500;

-- name: CompleteDiscussionJob :exec
update url_discussion_jobs
set scheduled_at    = $2,
    last_checked_at = now(),
    check_count     = $3,
    empty_count     = $4
where id = $1;

-- name: UpsertDiscussion :exec
insert into url_discussions (url_id, source, title, discussion_url, score, comment_count)
values ($1, $2, $3, $4, $5, $6)
on conflict (url_id, discussion_url) do update
	set score         = excluded.score,
	    comment_count = excluded.comment_count;

-- name: RecordBatchRun :exec
insert into discussion_run_log (started_at, url_count, found_count)
values ($1, $2, $3);

-- name: ListBatchRuns :many
select id, started_at, completed_at, url_count, found_count
from discussion_run_log
order by started_at desc
limit 15;

-- name: GetNextBatchRunAt :one
select scheduled_at from discussion_runs where id = 1;

-- name: ScheduleBatchRunNow :exec
update discussion_runs set scheduled_at = now() where id = 1;

-- name: ListDiscussionsForURL :many
select id, url_id, source, title, discussion_url, score, comment_count, found_at
from url_discussions
where url_id = $1
order by score desc, found_at desc;
