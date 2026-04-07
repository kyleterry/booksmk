alter table users add column theme text not null default 'dark' check (theme in ('dark', 'light', 'auto'));
