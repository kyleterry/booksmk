-- name: GetURL :one
select u.id, u.url, u.feed_url, uu.title, uu.description, uu.created_at, uu.updated_at,
  coalesce(array_agg(t.name order by t.name) filter (where t.name is not null), '{}')::text[] as tags
from urls u
join user_urls uu on uu.url_id = u.id
left join url_tags ut on ut.url_id = u.id and ut.user_id = uu.user_id
left join tags t on t.id = ut.tag_id
where u.id = $1 and uu.user_id = $2
group by u.id, u.url, u.feed_url, uu.title, uu.description, uu.created_at, uu.updated_at;

-- name: ListURLs :many
select u.id, u.url, u.feed_url, uu.title, uu.description, uu.created_at, uu.updated_at,
  coalesce(array_agg(t.name order by t.name) filter (where t.name is not null), '{}')::text[] as tags
from urls u
join user_urls uu on uu.url_id = u.id
left join url_tags ut on ut.url_id = u.id and ut.user_id = uu.user_id
left join tags t on t.id = ut.tag_id
where uu.user_id = $1
group by u.id, u.url, u.feed_url, uu.title, uu.description, uu.created_at, uu.updated_at
order by uu.created_at desc
limit $2 offset $3;

-- name: SearchURLs :many
select u.id, u.url, u.feed_url, uu.title, uu.description, uu.created_at, uu.updated_at,
  coalesce(array_agg(t.name order by t.name) filter (where t.name is not null), '{}')::text[] as tags
from urls u
join user_urls uu on uu.url_id = u.id
left join url_tags ut on ut.url_id = u.id and ut.user_id = uu.user_id
left join tags t on t.id = ut.tag_id
where uu.user_id = $1
  and (
    uu.title ilike @query
    or uu.description ilike @query
    or u.url ilike @query
  )
group by u.id, u.url, u.feed_url, uu.title, uu.description, uu.created_at, uu.updated_at
order by uu.created_at desc;

-- name: UpsertURL :one
insert into urls (url)
values ($1)
on conflict (url) do update set url = excluded.url
returning id;

-- name: AddURLToUser :exec
insert into user_urls (user_id, url_id, title, description)
values ($1, $2, $3, $4)
on conflict (user_id, url_id) do nothing;

-- name: UpdateUserURL :one
update user_urls
set title = $1, description = $2, updated_at = now()
where user_id = $3 and url_id = $4
returning url_id;

-- name: RemoveURLFromUser :exec
delete from user_urls where user_id = $1 and url_id = $2;

-- name: SetURLFeedURL :exec
update urls set feed_url = $2 where id = $1;

-- name: ListURLsByTag :many
with filtered_urls as (
  select ut2.url_id
  from url_tags ut2
  join tags t2 on t2.id = ut2.tag_id
  where ut2.user_id = $1 and t2.name = $2
)
select
  u.id, u.url, u.feed_url, uu.title, uu.description, uu.created_at, uu.updated_at,
  coalesce(array_agg(t.name order by t.name) filter (where t.name is not null), '{}')::text[] as tags
from urls u
join user_urls uu on uu.url_id = u.id
left join url_tags ut on ut.url_id = u.id and ut.user_id = uu.user_id
left join tags t on t.id = ut.tag_id
where uu.user_id = $1 and u.id in (select url_id from filtered_urls)
group by u.id, u.url, u.feed_url, uu.title, uu.description, uu.created_at, uu.updated_at
order by uu.created_at desc
limit $3 offset $4;

-- name: ListURLsByCategory :many
with filtered_urls as (
  select ut2.url_id from url_tags ut2
  join tags t2 on t2.id = ut2.tag_id
  join category_members cm on cm.kind = 'tag' and cm.value = t2.name
  where ut2.user_id = $1 and cm.category_id = $2
  union
  select u2.id from urls u2
  join category_members cm on cm.kind = 'domain'
  where cm.category_id = $2
    and regexp_replace(u2.url, '^https?://([^/?#]+).*$', '\1', 'i') = cm.value
)
select
  u.id, u.url, u.feed_url, uu.title, uu.description, uu.created_at, uu.updated_at,
  coalesce(array_agg(t.name order by t.name) filter (where t.name is not null), '{}')::text[] as tags
from urls u
join user_urls uu on uu.url_id = u.id
left join url_tags ut on ut.url_id = u.id and ut.user_id = uu.user_id
left join tags t on t.id = ut.tag_id
where uu.user_id = $1 and u.id in (select url_id from filtered_urls)
group by u.id, u.url, u.feed_url, uu.title, uu.description, uu.created_at, uu.updated_at
order by uu.created_at desc
limit $3 offset $4;

-- name: ListURLsForFeedBackfill :many
select id, url from urls where feed_url = '' order by created_at desc;
