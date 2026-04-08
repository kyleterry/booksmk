-- name: InsertCategory :one
insert into categories (user_id, name)
values ($1, $2)
returning *;

-- name: GetCategory :one
select * from categories where id = $1 and user_id = $2;

-- name: ListCategories :many
select * from categories where user_id = $1 order by name;

-- name: UpdateCategory :one
update categories set name = $1, updated_at = now()
where id = $2 and user_id = $3
returning *;

-- name: DeleteCategory :exec
delete from categories where id = $1 and user_id = $2;

-- name: InsertCategoryMember :exec
insert into category_members (category_id, kind, value)
values ($1, $2, $3)
on conflict do nothing;

-- name: DeleteAllCategoryMembers :exec
delete from category_members where category_id = $1;

-- name: ListCategoryMembers :many
select * from category_members where category_id = $1 order by kind, value;
