-- name: EnqueueDiscussionJob :exec
update urls set next_check_at = now() where id = $1;

-- name: ListDueURLs :many
select id, url, check_count, empty_count
from urls
where next_check_at <= now()
order by next_check_at
limit 500;

-- name: CompleteDiscussionJob :exec
update urls
set next_check_at   = $2,
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

-- name: ListDiscussionsForURL :many
select id, url_id, source, title, discussion_url, score, comment_count, found_at
from url_discussions
where url_id = $1
order by score desc, found_at desc;
