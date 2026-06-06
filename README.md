# Futemon

Futemon is a server-rendered web app for semi-comic futsal matches between teams of five Pokemon.

This first slice is intentionally small:

- Go `net/http` server
- Server-rendered HTML templates
- HTMX for partial updates
- Tailwind CDN for fast UI iteration
- SQLite persistence with demo seed data
- Deterministic simulation placeholders

## Run

```sh
go run ./cmd/server
```

Then open `http://localhost:8080`.

The server creates `futemon.db` in the working directory. Override it with:

```sh
FUTEMON_DB_PATH=/tmp/futemon.db go run ./cmd/server
```

To save Gemini BYOK credentials, configure a 32-byte encryption key:

```sh
ENV_ENCRYPTION_KEY=12345678901234567890123456789012 go run ./cmd/server
```

## Test

```sh
go test ./...
```

## Migrate And Seed

Apply migrations and demo seed data:

```sh
go run ./cmd/migrate --db futemon.db
```

Fetch Pokemon data from PokeAPI into the local cache:

```sh
go run ./cmd/migrate --db futemon.db --seed-pokemon --pokemon-limit 151
```

## Product Roadmap

1. Add create/edit/delete team flows backed by SQLite.
2. Expand synchronized broadcast timing from `start_time`.
3. Add Google OAuth2 sessions and role-based admin middleware.
4. Replace deterministic simulation with Gemini structured-output generation.
5. Add tournament bracket generation and consequence logs.
