# booksmk

A personal URL bookmarking service. Spiritual successor to [sufr](https://github.com/kyleterry/sufr).

## Features

- Save and organize URLs with titles, descriptions, and tags
- Automatic page title fetching when no title is provided

## Requirements

- Go 1.23+
- PostgreSQL 14+

## Development

The dev environment is managed with Nix. Enter the shell to get all tools and environment variables set:

```
nix develop
```

This sets `PGDATA`, `PGHOST`, and `BOOKSMK_DATABASE_URL`. PostgreSQL is managed manually — start it separately before running the app.

### Building

```
go build ./cmd/booksmk
```

### Testing

```
go test ./...
```

Store and migration tests require `BOOKSMK_DATABASE_URL` to be set and will be skipped otherwise.

### Code generation

Regenerate the sqlc query layer after changing SQL:

```
sqlc generate
```

Regenerate templ components after changing templates:

```
templ generate
```

## Configuration

| Variable | Default | Description |
|---|---|---|
| `BOOKSMK_DATABASE_URL` | required | PostgreSQL connection string |
| `BOOKSMK_ADDR` | `:8080` | Address and port to listen on |

## Stack

- [pgx](https://github.com/jackc/pgx) — PostgreSQL driver and connection pool
- [sqlc](https://sqlc.dev) — SQL query generation
- [templ](https://github.com/a-h/templ) — HTML templating
