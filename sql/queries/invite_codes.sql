-- name: CreateInviteCode :one
insert into invite_codes (code, created_by)
values ($1, $2)
returning *;

-- name: GetInviteCodeByCode :one
select * from invite_codes where code = $1;

-- name: ListInviteCodes :many
select * from invite_codes order by created_at desc;

-- name: UseInviteCode :exec
update invite_codes
set used_by = $1, used_at = now()
where id = $2;

-- name: DeleteInviteCode :exec
delete from invite_codes where id = $1;
