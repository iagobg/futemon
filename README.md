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

The development simulator reads `examples/sample_match.json` by default. Override it with:

```sh
FUTEMON_SAMPLE_MATCH_JSON=/path/to/match.json go run ./cmd/server
```

To enable Google OAuth login, create OAuth credentials in Google Cloud and set:

```sh
GOOGLE_CLIENT_ID=...
GOOGLE_CLIENT_SECRET=...
GOOGLE_REDIRECT_URL=http://localhost:8080/auth/google/callback
SESSION_SECRET=change-this-long-random-string
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

Fetch Pokemon data from PokeAPI into the local cache. This also mirrors official artwork PNGs into `data/pokemon-artwork` and stores `/static/pokemon-artwork/{id}.png` as the preferred image URL:

```sh
go run ./cmd/migrate --db futemon.db --seed-pokemon --pokemon-limit 151
```

Override the artwork directory with `FUTEMON_ARTWORK_DIR` or `--artwork-dir` when seeding. The server also reads `FUTEMON_ARTWORK_DIR` and serves `/static/pokemon-artwork/*` with a long immutable cache header.

## Product Roadmap

1. Add create/edit/delete team flows backed by SQLite.
2. Expand synchronized broadcast timing from `start_time`.
3. Add Google OAuth2 sessions and role-based admin middleware.
4. Replace deterministic simulation with Gemini structured-output generation.
5. Add tournament bracket generation and consequence logs.
