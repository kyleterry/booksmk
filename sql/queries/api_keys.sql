-- name: CreateAPIKey :one
insert into api_keys (user_id, name, token_hash, token_prefix, expires_at)
values ($1, $2, $3, $4, $5)
returning *;

-- name: ListAPIKeys :many
select * from api_keys
where user_id = $1
order by created_at desc;

-- name: CountAPIKeys :one
select count(*) from api_keys
where user_id = $1;

-- name: DeleteAPIKey :exec
delete from api_keys
where id = $1 and user_id = $2;

-- name: GetAPIKeyByTokenHash :one
select * from api_keys
where token_hash = $1
  and (expires_at is null or expires_at > now());
