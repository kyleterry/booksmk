# booksmk

A URL bookmarking and feed reader service. Spiritual successor to
[sufr](https://github.com/kyleterry/sufr).

## Screenshots

See [docs/screenshots.md](docs/screenshots.md) for what you're gonna get.

## Features

- Save and organize URLs with titles, descriptions, and tags
- Automatic page title and tag fetching
- Feed discovery and tracking for saved URLs
- API key authentication for some programmatic access

## Requirements

- Go 1.26+
- PostgreSQL 14+

## Development

Some task commands assume `podman` is available.

The dev environment is managed with Nix. Enter the shell to get all tools and
environment variables set:

```shell
nix develop # or: direnv allow
task db:start # starts a postgres container
task db:wait
task db:init
air # runs the server, rebuilds when files change.
```

### Building

```
go build ./cmd/booksmk ./cmd/booksmkctl
```

### Testing

```
task test
```

Store and migration tests require `BOOKSMK_DATABASE_URL` to be set and will be skipped otherwise.

### Code generation

Generate sqlc and templ code:

```
task generate
```

## Configuration

| Variable | Default | Description |
|---|---|---|
| `BOOKSMK_DATABASE_URL` | required | PostgreSQL connection string |
| `BOOKSMK_SECURE_COOKIES` | false | Enables secure cookies for production |
| `BOOKSMK_ADDR` | `:8080` | Address and port to listen on |

## Stack

- [pgx](https://github.com/jackc/pgx) — PostgreSQL driver and connection pool
- [sqlc](https://sqlc.dev) — SQL query generation
- [templ](https://github.com/a-h/templ) — HTML templating
- [htmx](https://htmx.org/) - Simple JS shit; favors server-side rendering, like me.
