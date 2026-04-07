alter table users add column font_size text not null default 'medium' check (font_size in ('small', 'medium', 'large'));
