# 17AN — Customer Intelligence Platform

Customer Intelligence Platform built for the **17 Agustus Coding Festival** challenge
(see [`CHALLENGE.md`](./CHALLENGE.md)): search, data quality, duplicate detection,
and cross-table profile lookups over a ~15M-row customer dataset.

- **Backend:** Go + [Fiber](https://gofiber.io) + [GORM](https://gorm.io), repository → service → handler layers
- **Frontend:** plain HTML/CSS/JS, no build step, no framework
- **Database:** PostgreSQL 16 (see the important note below — it is **not** started by this repo's `docker-compose.yaml`)
- **Observability:** OpenTelemetry tracing → Jaeger, request-ID + trace-ID correlated logging, Swagger/OpenAPI docs
- **Load testing:** k6

**Live deployment:** frontend `http://188.166.231.180:8080/` · API `http://188.166.231.180:3000` · Swagger `http://188.166.231.180:3000/swagger/index.html` · Jaeger UI `http://188.166.231.180:16686`

---

## ⚠️ Postgres is external — read this before running `docker compose up`

The 15M-row dataset lives in a **standalone Postgres container already running on the VPS**
(`~/db/docker-compose.yaml`, container `lomba_postgres`), loaded once from the ~4.8GB
`challenge_db_anonymized_v2.sql` dump. This repo's `docker-compose.yaml` deliberately does
**not** define, restart, or otherwise touch that container — it only manages `backend`,
`frontend`, and `jaeger`. The backend reaches Postgres through the host's published `5432`
port via `host.docker.internal`.

This means:
- Running `docker compose up` here **will not** import any data or start a database — it
  connects to whichever Postgres you point `DB_HOST`/`DB_PORT`/etc. at in `.env`.
- To run this against the live dataset, you need SSH access to the VPS (or your own copy of
  the same dump imported into any reachable Postgres 16 instance with the indexes in
  [`backend/migrations/001_indexes.sql`](./backend/migrations/001_indexes.sql) applied — see
  [`DATABASE_NOTES.md`](./DATABASE_NOTES.md) for why those indexes are not optional).
- If you don't have access to the dataset, you can still run the whole stack against an
  empty/local Postgres to verify the API boots correctly — `/api/health` will report
  `total_records: 0` and search/quality/duplicates endpoints will just return empty results,
  but every endpoint will respond with valid JSON.

## Quickstart

```bash
git clone https://github.com/SaiNanamii/17an.git
cd 17an
cp .env.example .env
# edit .env: point DB_HOST/DB_PORT/DB_USER/DB_PASSWORD/DB_NAME at a reachable Postgres 16
#            with the dataset loaded (see the warning above)
docker compose up -d --build
sleep 30
curl http://localhost:3000/api/health
```

Expected: `200` with `{"ok":true,"status":"ready","database":"connected","total_records":...}`.

Services after `docker compose up`:

| Service | URL | Notes |
|---|---|---|
| Backend API | `http://localhost:3000` | Fiber, see endpoint table below |
| Swagger UI | `http://localhost:3000/swagger/index.html` | Auto-generated from handler annotations |
| Frontend dashboard | `http://localhost:8080` | Plain HTML/JS, no build step |
| Jaeger UI | `http://localhost:16686` | Distributed tracing (service name `17an-backend`) |

### `.env` reference

```bash
DB_HOST=host.docker.internal   # or your Postgres host
DB_PORT=5432
DB_USER=lomba
DB_PASSWORD=change_me
DB_NAME=lomba_challenge
DB_MAX_OPEN_CONNS=60           # optional, defaults shown in docker-compose.yaml
DB_MAX_IDLE_CONNS=20
DB_CONN_MAX_LIFETIME_MINUTES=30
DB_CONN_MAX_IDLE_TIME_MINUTES=5
```

All connection-pool settings are configurable via environment variables — see
`backend/config/config.go`.

## API endpoints

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/health`, `/api/health` | Health check — DB connectivity (live ping) + cached total row count |
| `GET` | `/api/search?q=&type=&limit=&offset=` | Search by `email`\|`phone`\|`user_id` (exact) or `name` (fuzzy, pg_trgm word_similarity) |
| `GET` | `/api/quality` | Full data-quality report (missing/invalid/duplicate breakdown per field) |
| `GET` | `/api/metrics` | Slim quality shape: `duplicates`, `missing_fields`, `quality_score` |
| `POST` | `/api/duplicates` `{limit}` | Exact duplicate pairs (same email or phone) |
| `GET` | `/api/duplicates/find?method=ip_address&limit=` | Duplicate account clusters sharing an IP |
| `GET` | `/api/duplicates/:user_id` | Duplicate candidates for one specific user |
| `GET` | `/api/user-profile/:user_id` | Cross-table profile: user + order count/total spend + activity (4-table join) |
| `GET` | `/api/v1/users?page=&limit=` | Paginated user listing |
| `GET` | `/api/v1/users/:id` | Single user by ID |

Full request/response shapes: Swagger UI at `/swagger/index.html`, or
[`backend/docs/swagger.json`](./backend/docs/swagger.json).

Several endpoints (`/api/quality`, `/api/metrics`, `POST /api/duplicates`,
`/api/duplicates/find`) are backed by an in-process cache refreshed every 60s in a
background goroutine rather than recomputed per request — see
[`DATABASE_NOTES.md`](./DATABASE_NOTES.md) for why (full-table scans over 15M rows are too
slow to run synchronously on every call). Expect a `500 "not yet computed"` for the first
few seconds after a fresh start/restart while these warm up.

## Project layout

```
backend/
  main.go             entrypoint: middleware, tracing, routes wiring
  config/              env-driven configuration
  database/            GORM connection + pg_prewarm on startup
  models/               struct definitions mapped to the live schema
  repository/           raw-SQL data access (search, analytics/quality/duplicates, users)
  service/               business logic + in-process caching layer
  handlers/               HTTP handlers (Swagger-annotated)
  routes/                 route registration
  migrations/           001_indexes.sql — indexes + extensions applied to the live DB
  tracing/               OpenTelemetry setup
  docs/                  auto-generated Swagger output (swag init)
frontend/
  index.html            single-file dashboard (search / quality / duplicates / profile tabs)
k6/
  loadtest.js            load test script covering every backend route
docker-compose.yaml     backend + frontend + jaeger (NOT postgres, see warning above)
DATABASE_NOTES.md       schema notes, indexes, performance trade-offs
REPORT.md               k6 load test results + edge case testing
CHALLENGE.md            the original challenge brief
```

## Load testing

```bash
BASE_URL=http://188.166.231.180:3000 k6 run k6/loadtest.js
```

100 concurrent VUs for 60s against every endpoint, 5s timeout per request. Current results
(against the live deployment): **avg 331ms, 0.15% failure rate** — see
[`REPORT.md`](./REPORT.md) for the full breakdown, the tuning history, and edge case /
injection testing.

## CI/CD

- **`.github/workflows/ci.yml`** — on every push: `go build`/`go vet` for the backend,
  `docker compose config` validation, and a Docker build of both `backend` and `frontend`
  images.
- **`.github/workflows/deploy.yml`** — on push to `main`: SSHes into the VPS, `git pull`,
  and `docker compose up -d --build jaeger backend frontend` (explicitly excluding
  postgres — see the warning above).

## Development

```bash
cd backend
go build ./...
go vet ./...
go run main.go   # requires .env in this directory (see .env reference above)
```

Regenerate Swagger docs after changing handler annotations:

```bash
cd backend
swag init
```
