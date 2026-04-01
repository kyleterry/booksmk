insert into url_discussion_jobs (url_id)
select id from urls
on conflict (url_id) do nothing;
