-- name: GetUser :one
select * from users where id = $1;

-- name: GetUserByEmail :one
select * from users where email = $1;

-- name: ListUsers :many
select * from users order by created_at desc;

-- name: CreateUser :one
insert into users (email, password_digest)
values ($1, $2)
returning *;

-- name: UpdateUser :one
update users
set email = $1, updated_at = now()
where id = $2
returning *;

-- name: UpdateUserPassword :one
update users
set password_digest = $1, updated_at = now()
where id = $2
returning *;

-- name: DeleteUser :exec
delete from users where id = $1;
