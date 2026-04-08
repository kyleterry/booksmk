-- name: GetURL :one
select u.id, u.url, u.feed_url, uu.title, uu.description, uu.created_at, uu.updated_at
from urls u
join user_urls uu on uu.url_id = u.id
where u.id = $1 and uu.user_id = $2;

-- name: ListURLs :many
select u.id, u.url, u.feed_url, uu.title, uu.description, uu.created_at, uu.updated_at
from urls u
join user_urls uu on uu.url_id = u.id
where uu.user_id = $1
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
select u.id, u.url, u.feed_url, uu.title, uu.description, uu.created_at, uu.updated_at
from urls u
join user_urls uu on uu.url_id = u.id
join url_tags ut on ut.url_id = u.id and ut.user_id = uu.user_id
join tags t on t.id = ut.tag_id
where uu.user_id = $1 and t.name = $2
order by uu.created_at desc;

-- name: ListURLsByCategory :many
select distinct u.id, u.url, u.feed_url, uu.title, uu.description, uu.created_at, uu.updated_at
from urls u
join user_urls uu on uu.url_id = u.id
where uu.user_id = $1
  and (
    exists (
      select 1 from url_tags ut
      join tags t on t.id = ut.tag_id
      join category_members cm on cm.kind = 'tag' and cm.value = t.name
      where ut.user_id = $1 and ut.url_id = u.id and cm.category_id = $2
    )
    or exists (
      select 1 from category_members cm
      where cm.category_id = $2 and cm.kind = 'domain'
        and regexp_replace(u.url, '^https?://([^/?#]+).*$', '\1', 'i') = cm.value
    )
  )
order by uu.created_at desc;
