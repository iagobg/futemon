# Futemon

Futemon is a server-rendered Go web app for futsal matches between Pokemon teams, with HTMX interactions, SQLite persistence, and LLM-generated match narration through OpenRouter.

## Quick Start With Docker

Create a `.env` file:

```sh
cp .env.example .env
```

For the simplest internal setup, keep auth in local mode and add your OpenRouter API key:

```env
FUTEMON_AUTH_MODE=local
OPENROUTER_API_KEY=sk-or-...
ENV_ENCRYPTION_KEY=12345678901234567890123456789012
SESSION_SECRET=change-this-long-random-string
FUTEMON_DB_PATH=data/futemon.db
FUTEMON_ARTWORK_DIR=data/pokemon-artwork
```

Keys used in this quick start:

- `OPENROUTER_API_KEY`: create this in your OpenRouter account. It is the key used to call `openai/gpt-oss-120b:free` or whichever `OPENROUTER_MODEL` you configure.
- `ENV_ENCRYPTION_KEY`: local app secret used to encrypt saved BYOK API keys in SQLite. Use either a raw 32-character string, a 32-byte base64 value, or a 32-byte hex value.
- `SESSION_SECRET`: local app secret used to sign browser sessions. Use a long random string.
- `FUTEMON_DB_PATH`: keep this as `data/futemon.db` so local runs use `./data/futemon.db` and Docker uses `/app/data/futemon.db` through the container workdir.
- Relative `data/...` paths are portable between local and Docker because the Docker image runs from `/app`.

Build the Docker image:

```sh
docker build -t futemon .
```

(Optional) Create and seed the persistent database volume with the first 151 Pokemon:

By default the starting database only has the 13 Pokemon present in the example teams.

```sh
docker run --rm \
  --env-file .env \
  -v futemon-data:/app/data \
  futemon \
  /app/futemon-migrate --seed-pokemon --pokemon-limit 151
```

Run the server:

```sh
docker run --rm \
  --env-file .env \
  -p 8080:8080 \
  -v futemon-data:/app/data \
  futemon
```

Open `http://localhost:8080`.

The sample `.env` opts into `FUTEMON_AUTH_MODE=local` for a trusted/internal setup. If `FUTEMON_AUTH_MODE` is absent or empty, the app defaults to Google auth. With `FUTEMON_DB_PATH=data/futemon.db`, SQLite data is stored at `/app/data/futemon.db` in Docker and persists in the mounted `/app/data` volume. In local auth mode the app uses the seeded demo user and does not require Google OAuth.

If the seed prints `seeded #151 Mew` but the app still only shows the example Pokemon, check that migrations and the server are using the same `FUTEMON_DB_PATH`. The recommended shared value is `data/futemon.db`.

## Server Flags

```sh
go run ./cmd/server -auth-mode local -port 8080 -db data/futemon.db
```

Flags:

- `-auth-mode`: `local` or `google`.
- `-port`: HTTP port.
- `-db`: SQLite database path.

Environment variables still work and are used as defaults for these flags.

## Auth Modes

### Local

```env
FUTEMON_AUTH_MODE=local
```

- No Google OAuth.
- Uses the seeded demo user.
- Best for local/internal use.
- Daily duel limit defaults to disabled (`0`) in this mode.

### Google

```env
FUTEMON_AUTH_MODE=google
GOOGLE_CLIENT_ID=...
GOOGLE_CLIENT_SECRET=...
GOOGLE_REDIRECT_URL=http://localhost:8080/auth/google/callback
SESSION_SECRET=change-this-long-random-string
```

Google mode is the default when `FUTEMON_AUTH_MODE` is absent.

## LLM And OpenRouter

Create an API key in OpenRouter and set it as:

```env
OPENROUTER_API_KEY=sk-or-...
```

Default model:

```env
OPENROUTER_MODEL=openai/gpt-oss-120b:free
```

Useful options:

```env
OPENROUTER_API_KEY=sk-or-...
OPENROUTER_BASE_URL=https://openrouter.ai/api/v1
OPENROUTER_TIMEOUT_SECONDS=120
FUTEMON_LLM_PROMPT_PATH=examples/systemprompt.md
FUTEMON_LLM_DISABLED=0
FUTEMON_LLM_FALLBACK_ON_ERROR=0
FUTEMON_LLM_STRICT_SCHEMA=0
```

Notes:

- If `OPENROUTER_API_KEY` is absent, the app uses the local sample simulation.
- If `FUTEMON_LLM_FALLBACK_ON_ERROR=1`, LLM failures fall back to the sample simulation.
- By default, LLM failures are returned as errors so they can be diagnosed from server logs.
- `FUTEMON_LLM_STRICT_SCHEMA=1` sends `response_format: json_schema`; leave it off if the selected model/provider rejects strict structured output.
- The current model handles the shorter systemprompt (systemprompt.md) better, depending on the model you're using you may have better results with systemprompt_longer.md

## Rate Limit And BYOK

```env
FUTEMON_DAILY_DUEL_LIMIT=1
```

- In Google mode, users default to 1 completed duel per day.
- In local mode, the default is `0`, meaning no local daily limit.
- Daily duel usage is persisted in SQLite by user and UTC date.
- Users can save their own OpenRouter key in account settings. When present, that BYOK key is used for duel generation and bypasses the local daily limit.
- Saved BYOK API keys are OpenRouter API keys.
- Saved API keys require `ENV_ENCRYPTION_KEY` to resolve to exactly 32 bytes so they can be encrypted at rest. Accepted formats are a raw 32-character string, `base64:<base64 value>`, or `hex:<64 hex characters>`.

## Team Transfers

- Each team has one weekly Pokemon transfer window.
- A valid weekly transfer can change exactly one Pokemon. Team name and abilities can be edited together with that transfer.
- Transfer usage is tracked in `team_transactions` by `window_start`, preserving the team's formation and transfer history.

## Data And Seeding

The server creates and migrates the SQLite database automatically.

The same relative `.env` paths work locally and in Docker. For the default setup, both commands below use `data/futemon.db`.

To run the migration command manually:

```sh
go run ./cmd/migrate
```

Fetch Pokemon data and official artwork:

```sh
go run ./cmd/migrate --seed-pokemon --pokemon-limit 151
```

Artwork is served from:

```env
FUTEMON_ARTWORK_DIR=data/pokemon-artwork
```

The local sample simulation can be overridden:

```env
FUTEMON_SAMPLE_MATCH_JSON=examples/sample_match.json
```

## Test

```sh
go test ./...
```
