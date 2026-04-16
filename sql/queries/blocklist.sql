-- name: CreateBlocklistEntry :one
insert into blocklist (pattern, kind)
values ($1, $2)
returning *;

-- name: DeleteBlocklistEntry :exec
delete from blocklist
where id = $1;

-- name: ListBlocklistEntries :many
select * from blocklist
order by created_at desc;

-- name: GetBlocklistEntryByPattern :one
select * from blocklist
where pattern = $1;

-- name: IsBlocked :one
select exists (
    select 1 from blocklist
    where (kind = 'url' and pattern = $1)
       or (kind = 'domain' and ($2::text = pattern or $2::text like '%.' || pattern))
) as blocked;
