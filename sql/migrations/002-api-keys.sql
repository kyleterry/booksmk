create table api_keys (
	id           uuid        primary key default gen_random_uuid(),
	user_id      uuid        not null references users(id) on delete cascade,
	name         text        not null,
	token_hash   text        not null unique,
	token_prefix text        not null,
	expires_at   timestamptz,
	created_at   timestamptz not null default now()
);
