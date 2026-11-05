# Subscription Hub

> Open-source subscription and resource lifecycle manager (V1 in progress).

[English](README.md) | [简体中文](README.zh-CN.md)

Manage recurring services, VPS subscriptions, domains, and other upcoming renewals in one self-hosted dashboard.

## Implemented in V1 phase 1

- SQLite persistence with schema versioning
- Category management and subscription/resource CRUD
- Day, month, and year recurrence anchored to the original renewal date
- Month-end clamping and leap-year handling
- Basic statuses: active, paused, canceled
- Responsive Vue interface for browsing, creating, editing and deleting records
- Data persistence through a Docker volume
- Actual payment ledger (CRUD) and six-month currency-separated expense reports
- Idempotent in-app renewal reminders, configurable 0–30 days ahead

**Not implemented yet:** authentication, external notification delivery (email/Bark/Telegram), refunds, currency exchange, backups, AI, MCP. This is a **local-development preview**, not suitable for public internet access.

## Technology

- Backend: Go 1.26 + Gin + modernc.org/sqlite (pure Go)
- Frontend: Vue 3 + TypeScript + Vite
- Storage: SQLite (WAL mode)
- Deployment: Docker Compose for local development

## Quick start

### Docker Compose (recommended)

```bash
docker compose up --build
```

Open http://localhost:5173. The SQLite database is stored in the named Docker volume `subscription_data`. `docker compose down` keeps it, but `docker compose down -v` permanently deletes the volume.

### Local development

Requires Go 1.26+, Node.js 22.18+, and npm.

Backend:

```bash
cd backend
go mod tidy
go run ./cmd/server
```

In another terminal:

```bash
cd frontend
npm install
npm run dev
```

Open http://localhost:5173. Backend listens on `127.0.0.1:8080` by default.

Configuration:

| Variable | Default | Description |
|---|---|---|
| `HOST` | `127.0.0.1` | API binding address |
| `PORT` | `8080` | HTTP port |
| `DB_PATH` | `./data/subscription-hub.db` | SQLite file path |
| `APP_TIMEZONE` | `Asia/Shanghai` | Calendar timezone for next-due calculations |

The Docker Compose backend binds to `0.0.0.0` **inside the container**, but is published on `127.0.0.1` on the Docker host.

## API

See [docs/API.md](docs/API.md) for catalog endpoints, and [docs/FINANCE.md](docs/FINANCE.md) for payments, reports and in-app reminders.

## Roadmap

See [docs/ROADMAP.md](docs/ROADMAP.md).

## Security

The API has **no authentication**. Do **not** expose port 8080 or the frontend proxy to untrusted networks. Back up the database before upgrade or removal. Avoid storing API secrets in notes.

## License

MIT. See [LICENSE](LICENSE).
