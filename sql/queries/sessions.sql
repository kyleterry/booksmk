-- name: CreateSession :one
insert into sessions (token, user_id, expires_at)
values ($1, $2, $3)
returning *;

-- name: GetSession :one
select * from sessions where token = $1 and expires_at > now();

-- name: GetSessionUser :one
select u.*
from users u
join sessions s on s.user_id = u.id
where s.token = $1 and s.expires_at > now();

-- name: DeleteSession :exec
delete from sessions where token = $1;

-- name: DeleteUserSessions :exec
delete from sessions where user_id = $1;
