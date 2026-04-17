-- name: UpsertFeed :one
insert into feeds (feed_url, is_blocked_bypass)
values ($1, $2)
on conflict (feed_url) do update set 
    is_blocked_bypass = excluded.is_blocked_bypass or feeds.is_blocked_bypass
returning id, feed_url, site_url, title, description, image_url, is_blocked_bypass, last_fetched_at, created_at, updated_at;

-- name: GetFeedByID :one
select id, feed_url, site_url, title, description, image_url, is_blocked_bypass, last_fetched_at, created_at, updated_at
from feeds
where id = $1;

-- name: UpdateFeedMeta :exec
update feeds
set site_url        = $2,
    title           = $3,
    description     = $4,
    image_url       = $5,
    last_fetched_at = now(),
    updated_at      = now()
where id = $1;

-- name: AddFeedToUser :exec
insert into user_feeds (user_id, feed_id)
values ($1, $2)
on conflict (user_id, feed_id) do nothing;

-- name: RemoveFeedFromUser :exec
delete from user_feeds where user_id = $1 and feed_id = $2;

-- name: UpdateUserFeedCustomName :exec
update user_feeds set custom_name = $3 where user_id = $1 and feed_id = $2;

-- name: GetUserFeed :one
select f.id, f.feed_url, f.site_url, f.title, f.description, f.image_url, f.is_blocked_bypass, f.last_fetched_at, f.created_at, f.updated_at, uf.custom_name
from feeds f
join user_feeds uf on uf.feed_id = f.id
where f.id = $1 and uf.user_id = $2;

-- name: GetUserFeedByFeedURL :one
select f.id, f.feed_url, f.site_url, f.title, f.description, f.image_url, f.is_blocked_bypass, f.last_fetched_at, f.created_at, f.updated_at, uf.custom_name
from feeds f
join user_feeds uf on uf.feed_id = f.id
where f.feed_url = $1 and uf.user_id = $2;

-- name: ListUserFeeds :many
select f.id, f.feed_url, f.site_url, f.title, f.description, f.image_url, f.is_blocked_bypass, f.last_fetched_at, f.created_at, f.updated_at, uf.custom_name
from feeds f
join user_feeds uf on uf.feed_id = f.id
where uf.user_id = $1
order by f.title, f.created_at desc;

-- name: AddTagToFeed :exec
insert into feed_tags (user_id, feed_id, tag_id) values ($1, $2, $3)
on conflict do nothing;

-- name: RemoveAllTagsFromFeed :exec
delete from feed_tags where user_id = $1 and feed_id = $2;

-- name: ListTagNamesForFeed :many
select t.name from tags t
join feed_tags ft on ft.tag_id = t.id
where ft.user_id = $1 and ft.feed_id = $2
order by t.name;

-- name: UpsertFeedItem :one
insert into feed_items (feed_id, guid, url, title, summary, author, published_at)
values ($1, $2, $3, $4, $5, $6, $7)
on conflict (feed_id, guid) do update
    set url          = excluded.url,
        title        = excluded.title,
        summary      = excluded.summary,
        author       = excluded.author,
        published_at = excluded.published_at
returning id;

-- name: ListFeedItems :many
select fi.id, fi.feed_id, fi.guid, fi.url, fi.title, fi.summary, fi.author, fi.published_at, fi.created_at,
       exists (
         select 1 from feed_item_reads fir
         where fir.item_id = fi.id and fir.user_id = $2
       ) as is_read
from feed_items fi
where fi.feed_id = $1
  and (fi.published_at is null or fi.published_at <= now())
order by fi.published_at desc nulls last
limit 100;

-- name: ListTimelineItems :many
with timeline as (
  select
    fi.id, fi.feed_id, fi.guid, fi.url, fi.title as item_title,
    fi.summary, fi.author, fi.published_at, fi.created_at,
    f.title as feed_title, f.image_url as feed_image_url,
    uf.custom_name as feed_custom_name
  from feed_items fi
  join feeds f on f.id = fi.feed_id
  join user_feeds uf on uf.feed_id = fi.feed_id and uf.user_id = $1
  where fi.published_at is null or fi.published_at <= now()
  order by fi.published_at desc nulls last
  limit $2
  offset $3
)
select
  t.id, t.feed_id,
  coalesce(nullif(t.feed_custom_name, ''), t.feed_title) as feed_title,
  t.feed_image_url,
  t.guid, t.url, t.item_title,
  t.summary, t.author, t.published_at, t.created_at,
  exists (
    select 1 from feed_item_reads fir
    where fir.item_id = t.id and fir.user_id = $1
  ) as is_read
from timeline t;

-- name: GetTimelineItem :one
with item as (
  select
    fi.id, fi.feed_id, fi.guid, fi.url, fi.title as item_title,
    fi.summary, fi.author, fi.published_at, fi.created_at,
    f.title as feed_title, f.image_url as feed_image_url,
    uf.custom_name as feed_custom_name
  from feed_items fi
  join feeds f on f.id = fi.feed_id
  join user_feeds uf on uf.feed_id = fi.feed_id and uf.user_id = $1
  where fi.id = $2
)
select
  i.id, i.feed_id,
  coalesce(nullif(i.feed_custom_name, ''), i.feed_title) as feed_title,
  i.feed_image_url,
  i.guid, i.url, i.item_title,
  i.summary, i.author, i.published_at, i.created_at,
  exists (
    select 1 from feed_item_reads fir
    where fir.item_id = i.id and fir.user_id = $1
  ) as is_read
from item i;

-- name: MarkItemRead :exec
insert into feed_item_reads (user_id, item_id)
values ($1, $2)
on conflict do nothing;

-- name: MarkItemUnread :exec
delete from feed_item_reads where user_id = $1 and item_id = $2;

-- name: MarkAllItemsRead :exec
insert into feed_item_reads (user_id, item_id)
select $1, fi.id
from feed_items fi
join user_feeds uf on uf.feed_id = fi.feed_id and uf.user_id = $1
where fi.published_at is null or fi.published_at <= now()
on conflict do nothing;

-- name: MarkFeedItemsRead :exec
insert into feed_item_reads (user_id, item_id)
select $1, fi.id
from feed_items fi
join user_feeds uf on uf.feed_id = fi.feed_id and uf.user_id = $1
where fi.feed_id = $2
  and (fi.published_at is null or fi.published_at <= now())
on conflict do nothing;

-- name: EnqueueFeedPollJob :exec
update feeds set next_fetch_at = now() where id = $1;

-- name: ListDueFeedPollJobs :many
select id, feed_url, fetch_count, error_count
from feeds
where next_fetch_at <= now()
order by next_fetch_at
limit 200;

-- name: CompleteFeedPollJob :exec
update feeds
set next_fetch_at   = $2,
    last_fetched_at = now(),
    fetch_count     = $3,
    error_count     = $4,
    last_error      = $5
where id = $1;

-- name: ListFeedPollJobStatuses :many
select id,
       feed_url,
       title,
       next_fetch_at as scheduled_at,
       last_fetched_at,
       fetch_count,
       error_count,
       last_error
from feeds
order by next_fetch_at asc
limit 100;

-- name: ScheduleAllFeedPollJobsNow :exec
update feeds set next_fetch_at = now();

-- name: SearchFeeds :many
select f.id, f.feed_url, f.site_url, f.title, f.description, f.image_url, f.is_blocked_bypass, f.last_fetched_at, f.created_at, f.updated_at, uf.custom_name
from feeds f
join user_feeds uf on uf.feed_id = f.id
where uf.user_id = $1
  and (
    f.title ilike @query
    or f.description ilike @query
    or f.feed_url ilike @query
    or uf.custom_name ilike @query
  )
order by f.title, f.created_at desc;
