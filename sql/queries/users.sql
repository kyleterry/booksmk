-- name: GetUser :one
select * from users where id = $1;

-- name: GetUserByEmail :one
select * from users where email = $1;

-- name: ListUsers :many
select id, email, is_admin, created_at from users order by created_at desc;

-- name: CreateUser :one
insert into users (email, password_digest, is_admin)
values ($1, $2, $3)
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

-- name: UpdateUserSettings :one
update users
set theme = $1, font_size = $2, results_per_page = $3, updated_at = now()
where id = $4
returning *;

-- name: CountUsers :one
select count(*) from users;
