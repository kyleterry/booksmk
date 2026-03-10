-- name: GetURL :one
select u.id, u.url, uu.title, uu.description, uu.created_at, uu.updated_at
from urls u
join user_urls uu on uu.url_id = u.id
where u.id = $1 and uu.user_id = $2;

-- name: ListURLs :many
select u.id, u.url, uu.title, uu.description, uu.created_at, uu.updated_at
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
