# Subscription Hub

> 轻量、开源的订阅与资源到期管理平台（开发中）。

[English](README.md) | [简体中文](README.zh-CN.md)

Subscription Hub helps individuals and developers track recurring subscriptions, VPS and domain expirations, payment history, and renewal reminders in one self-hosted dashboard.

## Status

**Early development / bootstrap.** The repository currently contains a runnable API health check and a frontend starter. Subscription billing, storage, and notifications are planned, not yet implemented.

## Technology

- Backend: Go + Gin
- Frontend: Vue 3 + TypeScript + Vite
- Planned persistence: SQLite; MySQL support later
- Deployment: Docker Compose (development bootstrap)

## Quick start

### Requirements

- Go 1.23+
- Node.js 22.18+ and npm
- Docker with Compose (optional)

### Local development

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

Visit http://localhost:5173. The frontend proxies /api to the backend on port 8080.

### Docker Compose

```bash
docker compose up --build
```

Visit http://localhost:5173. This bootstrap Compose configuration is for **development**, not a hardened public deployment.

### Smoke test

```bash
curl http://localhost:8080/api/v1/health
```

Expected response: `{"status":"ok"}`.

## Roadmap

- [ ] Subscription and resource CRUD (VPS, domains, memberships)
- [ ] Recurrence rules and next-renewal calculation
- [ ] Payment records and expense dashboard
- [ ] Notification adapters (SMTP, Telegram, Bark, Webhook)
- [ ] SQLite persistence, schema migrations and backups
- [ ] Authentication, authorization and rate limiting
- [ ] Currency conversion and historical reporting
- [ ] AI import and MCP integration

See [docs/ROADMAP.md](docs/ROADMAP.md) for scope and acceptance criteria.

## Repository structure

```text
backend/          Go API
frontend/         Vue 3 app
docs/             Architecture and roadmap
.github/workflows/ Continuous integration
```

## Security

Do not expose this starter directly to the public internet. Authentication, protected data storage and production deployment safeguards have not been implemented yet. Do not commit secrets.

## License

MIT. See [LICENSE](LICENSE).
