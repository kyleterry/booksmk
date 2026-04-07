create table feeds (
    id              uuid        primary key default gen_random_uuid(),
    feed_url        text        not null unique,
    site_url        text        not null default '',
    title           text        not null default '',
    description     text        not null default '',
    last_fetched_at timestamptz,
    created_at      timestamptz not null default now(),
    updated_at      timestamptz not null default now()
);

create table user_feeds (
    user_id    uuid        not null references users(id) on delete cascade,
    feed_id    uuid        not null references feeds(id) on delete cascade,
    created_at timestamptz not null default now(),
    primary key (user_id, feed_id)
);

create table feed_tags (
    user_id uuid not null references users(id) on delete cascade,
    feed_id uuid not null references feeds(id) on delete cascade,
    tag_id  uuid not null references tags(id) on delete cascade,
    primary key (user_id, feed_id, tag_id)
);

create table feed_items (
    id           uuid        primary key default gen_random_uuid(),
    feed_id      uuid        not null references feeds(id) on delete cascade,
    guid         text        not null,
    url          text        not null default '',
    title        text        not null default '',
    summary      text        not null default '',
    author       text        not null default '',
    published_at timestamptz,
    created_at   timestamptz not null default now(),
    unique (feed_id, guid)
);

create table feed_item_reads (
    user_id uuid        not null references users(id) on delete cascade,
    item_id uuid        not null references feed_items(id) on delete cascade,
    read_at timestamptz not null default now(),
    primary key (user_id, item_id)
);

create table feed_poll_jobs (
    id              uuid        primary key default gen_random_uuid(),
    feed_id         uuid        not null references feeds(id) on delete cascade unique,
    scheduled_at    timestamptz not null default now(),
    last_fetched_at timestamptz,
    fetch_count     int         not null default 0,
    error_count     int         not null default 0,
    last_error      text        not null default ''
);

create index on feed_items (feed_id, published_at desc);
create index on feed_poll_jobs (scheduled_at);
