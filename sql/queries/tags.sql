-- name: UpsertTag :one
insert into tags (name) values ($1)
on conflict (name) do update set name = excluded.name
returning *;

-- name: AddTagToURL :exec
insert into url_tags (user_id, url_id, tag_id) values ($1, $2, $3)
on conflict do nothing;

-- name: RemoveAllTagsFromURL :exec
delete from url_tags where user_id = $1 and url_id = $2;

-- name: ListTagNamesForURL :many
select t.name from tags t
join url_tags ut on ut.tag_id = t.id
where ut.user_id = $1 and ut.url_id = $2
order by t.name;

-- name: SetURLTags :exec
with new_tags as (
  insert into tags (name)
  select unnest(@names::text[])
  on conflict (name) do update set name = excluded.name
  returning id
)
insert into url_tags (user_id, url_id, tag_id)
select $1, $2, id from new_tags
on conflict do nothing;
