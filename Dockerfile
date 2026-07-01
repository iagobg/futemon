FROM node:20-bookworm-slim AS assets

WORKDIR /src
COPY package.json package-lock.json tailwind.config.js ./
COPY internal/app ./internal/app
RUN npm ci \
  && npm run build:css

FROM golang:1.22-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=assets /src/internal/app/static/app.css ./internal/app/static/app.css
RUN CGO_ENABLED=1 GOOS=linux go build -o /out/futemon ./cmd/server \
  && CGO_ENABLED=1 GOOS=linux go build -o /out/futemon-migrate ./cmd/migrate

FROM debian:bookworm-slim

RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=build /out/futemon /app/futemon
COPY --from=build /out/futemon-migrate /app/futemon-migrate
COPY examples /app/examples
COPY data /app/data

ENV PORT=8080
ENV FUTEMON_DB_PATH=/app/data/futemon.db
ENV FUTEMON_ARTWORK_DIR=/app/data/pokemon-artwork

EXPOSE 8080
VOLUME ["/app/data"]

CMD ["/app/futemon"]
