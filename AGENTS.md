# Veer AGENTS.md

## Three binaries, three entrypoints

| Binary     | Entrypoint              | Port | Purpose                    |
|------------|-------------------------|------|----------------------------|
| `veer`     | `backend/cmd/manager`   | 8080 | Manager: API only          |
| `scheduler`| `backend/cmd/scheduler` | 8081 | Data-plane: 302 redirects  |
| `edge`     | `backend/cmd/edge`      | 8082 | HTTP cache proxy           |

## Build

```sh
./build.sh                          # production build: manager binary
cd backend && go build ./cmd/manager # manager binary (API only)
go build -o scheduler ./cmd/scheduler
go build -o edge ./cmd/edge
```

## Dev

- Manager: `cd backend && go run ./cmd/manager`
- Frontend: `cd frontend && npm install && npm run dev` (port 5173, proxies `/api` to backend:8080)
- Config: `backend/config.yaml` (Viper, env prefix `CDNC_`)

## Tests

Zero tests exist. No `_test.go` files in the repo.

## Key conventions

- Module path: `veer` (not a path-based module)
- Go 1.21, CGO_ENABLED=1 required (SQLite via GORM)
- Config priority: env var > `config.yaml` > code default
- DB: SQLite, auto-migrates on startup, seeds 3 nodes + 2 rules if empty
- Default admin: `admin` / `admin123` (from config)
- JWT stored in frontend `localStorage` key `veer_token`
- Rate limit: sliding window, 60 req/min/IP (whitelist: 127.0.0.1, ::1)
- API auth: all `/api/*` except `POST /api/auth/login` require JWT Bearer token
- CORS allows only `http://localhost:5173`
- Health check Goroutine: pings every 30s, 3 consecutive fails → `inactive`
- Docker compose: manager + scheduler + edge-1 + edge-2

## Edge node

Registers with manager on startup via `CDNC_EDGE_MANAGER_URL`, syncs rules every 60s. Acts as HTTP cache proxy. If manager unreachable, runs with local config.

## Architecture doc

`deliverables/arch/incremental-arch-v2.md` — not always in sync with code, but covers the system design. Prefer reading code over the doc for ground truth.
