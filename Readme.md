# NTech

[🇷🇸 Srpska verzija](Readme_sr.md)

![Go Version](https://img.shields.io/badge/go-1.26-blue)
![License](https://img.shields.io/badge/license-MIT-green)

<img width="1440" height="754" alt="image" src="https://github.com/user-attachments/assets/2ea0746f-2f8e-46f7-9551-06e49c0e61ed" />

A business application for computer repair shop management, parts inventory tracking, and sales. Built with Go, it runs in the browser and requires no internet connection or external services.

> ⚠️ The project is under active development. It is not ready for production use.

**Live demo:** [https://demo.vm-net.in.rs](https://demo.vm-net.in.rs) — log in with `Demo` / `Demo1234` (admin role; password and 2FA changes are disabled in demo mode).

---

## About the Project

NTech is an internal application built for a specific user — a computer repair shop that, in addition to repairs, manages a parts inventory, sales of components and pre-built configurations, as well as client and supplier records.

The goal is simple: everything the repair shop needs to track is located in one place, without relying on Excel spreadsheets or paper records.

---

## How It Works

**Server-rendered, no build step.** Pages are rendered on the server with Go's `html/template` (auto-escaping, no client-side templating engine). [HTMX](https://htmx.org) swaps page fragments over plain HTTP for SPA-like navigation, and [Alpine.js](https://alpinejs.dev) handles small bits of client-side interactivity (live totals, dynamic form rows, autocomplete). There is no `npm`/webpack build — the browser gets plain HTML/CSS/JS, and a single page load is enough to use the whole app.

**Single binary, no runtime dependencies.** Templates, static assets (CSS/JS/images) and SQL migrations are embedded into the compiled binary with `go:embed`. Deploying is copying one file (or running one Docker image) — there's no separate asset build, no template files to ship alongside the executable. In development the same code reads straight from disk instead, so template/CSS/JS edits are visible on refresh without a rebuild.

**Request flow.** `chi` routes each request through a middleware chain — security headers → CSRF (double-submit cookie) → session lookup → role/permission check — before it reaches a handler. Permission checks run at the router level (so a route can't accidentally ship unprotected) and again inside the handler as defense in depth. Handlers talk to the database through a repository layer (plain SQL, no ORM) and pass plain Go structs to templates.

**Database.** SQLite via a pure-Go driver (`modernc.org/sqlite`, no CGO) is the default and only dependency — a single file, no separate database server to run or back up. Every startup applies any new SQL migration files in order and records them in a `migracije` table, so upgrades are just "replace the binary and restart." An optional PostgreSQL backend (via `pgx/v5`) is planned for multi-user setups.

**Client-facing public pages.** Two flows don't require a login, only a unique unguessable token in the URL: the service-status page (a client can check repair progress and get a QR code straight from their receipt) and the parts/service proposal approval page (a client accepts or rejects an estimate with a comment). Both are capability-based — whoever has the link has access to that one order, nothing else.

**Printable documents.** Work orders, pre-invoices, dispatch notes, return slips, and device labels are separate, self-contained HTML pages styled for A4 printing (`@media print`), each with a "Print" button that opens the browser's native print dialog — no PDF library, no headless-browser rendering step. Documents that don't fit one page paginate themselves client-side (measuring content height and inserting page breaks with running page numbers).

---

## Features

### Implemented

- Initial setup on first run (setup wizard)
- Database migration system
- User interface — sidebar navigation, theme system (dark/light), dashboard with statistics
- User login — server-side sessions, account locking
- Two-factor authentication (TOTP) — activation with a QR code; secret encrypted at rest (AES-256-GCM, key kept outside the database)
- Backup (one-time) codes for 2FA — generated on activation, stored as bcrypt hashes; a fallback to TOTP at login
- Brute-force protection — IP locking after 5 failed attempts within 15 minutes
- CSRF protection — double-submit cookie pattern, automatic token injection into all forms
- Security HTTP headers (CSP, X-Frame-Options, Referrer-Policy, nosniff...)
- Login attempt logging — history by user, IP, reason, date
- Users and roles — admin panel, user management
- Inventory — items, categories, filtering, critical stock levels, per-item stock card, supplier links, item transfers
- Barcode (EAN) per item — searchable by barcode in inventory; in the sales screen, scanning a barcode (any USB/Bluetooth scanner that types + Enter) looks the item up and adds it to the order automatically
- Service orders:
  - Intake form, status bar, archive
  - Diagnostic workflow — fault description, technician notes, work done, diagnostic fee
  - Parts and services — used items deducted from stock; suggested items (proposal to client)
  - Client proposal approval — client receives a public link (QR code) to accept or reject a parts/service proposal with a comment
  - Public status tracking via QR code — the device label and every printed document carry a QR code the client scans with their phone; it opens a mobile-optimized page (no login, no app) showing the current repair status, which they can revisit any time by the same link/code
  - Documents — work order, pre-invoice (estimate), dispatch note, return slip, device label (QR + Code128 barcode)
  - Pickup with payment — tracks payment method and advance amount
  - Guarantee period, expected completion date, technician assignment, client notes
- Sales orders — items, calculation, receipt with company and client details
- Services catalog — service price list used for billing in service orders
- Expenses — expense records with category and amount
- Procurement — records of purchases from suppliers
- Sales price calculation on procurement — markup (global, per category, per item), landed costs (customs, shipping...) allocated across items, two-way markup↔price computation; respects VAT-payer status
- Price revaluation (nivelacija) — sales price changes with an audit trail (old→new, reason, source, user)
- Company profile and modules — features toggle based on company type and VAT-payer status
- VAT records (KIR/KPR) — books of issued and received invoices, auto-filled from sales and procurement
- VAT calculation per period + mapping to the PP-PDV form; imports (customs declaration) tracked in fields 006/106
- VAT rate code list
- **Fiscalization (ESIR/L-PFR)** — full Go client for the Teron fiscal device API: connection test, invoice issuing (sale/service, including advances and refunds on cancellation), daily till summary, end-of-day closure, PDF fiscal reports, QR-code invoice verification (public `/v/` page), automatic retry on failed fiscalization with a visible error state, and a card-emulator status/reset panel (talks to the signing device over TCP). Currently verified against the Teron mock server (`Fisk/`); not yet tested against real certified hardware.
- Clients and suppliers — contact database
- Reminders — records with deadlines
- Reports — revenue overview, inventory status, inventory value report, stock movement list, stocktake (physical count)
- Settings — company name, address, Tax ID (PIB), logo; theme toggle
- Background images — login page and app, with blur, transparency and glass effect
- Personal theme and background — each user can set their own theme and background image
- Permission matrix (RBAC) — admin panel for permissions by role; enforced at the route level (both mutations and views) and in handlers
- Flash messages — one-time feedback after an action
- Automatic SQLite backup — with configurable number of retained copies; restore from a copy (safe, with no downtime)
- Charts — monthly revenue on reports (Chart.js)
- Structured logging — `log/slog` (JSON in production, text in development); separate auth log in fail2ban format
- Automated tests — unit and integration over a SQLite database (crypto, RBAC, login flows, form validators, reports)
- **Demo mode** (`NTECH_ENV=demo`) — auto-created demo user, pre-filled login form, restricted backup count, blocked password/2FA changes

### In Progress

- **Fiscalization on real hardware** — the full flow (issuing, refunds, daily till, closure, reports) is implemented and verified against the Teron mock server; testing against a real certified L-PFR device is still pending.

### Planned

- KPO book and double-entry bookkeeping (optional, later phase)
- PostgreSQL support (for multi-user environments)
- WebAuthn / Passkey login (database schema is already prepared)
- Notifications (email / WhatsApp) — deferred to a later phase
- Barcode scanning via camera — deferred to a later phase

---

## Technologies

| Technology                                                                           | Role                            |
| ------------------------------------------------------------------------------------ | ------------------------------- |
| [Go](https://go.dev)                                                                 | backend language                |
| [chi](https://github.com/go-chi/chi)                                                 | HTTP router                     |
| [html/template](https://pkg.go.dev/html/template)                                    | server-side templates           |
| [HTMX](https://htmx.org)                                                             | dynamic HTML over HTTP          |
| [Alpine.js](https://alpinejs.dev)                                                    | client-side UI logic            |
| [SQLite](https://sqlite.org) + [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) | main database (pure Go, no CGO) |
| [PostgreSQL](https://www.postgresql.org) + [pgx/v5](https://github.com/jackc/pgx)    | optional production database    |

---

## Running the Application

### Requirements

- Go 1.26 or newer
- Git

### Steps

```bash
# 1. Clone the repository
git clone <repository-url>
cd GoNtech

# 2. Run in development mode (reads files from disk, no HTTPS required)
go run ./cmd/ntech
```

The application opens at `http://localhost:8080`. On first run the setup wizard starts automatically.

### Production Build

Use the interactive build script:

```bash
./start.sh
```

It asks for the version, environment (production/development), platform (Linux/Windows/both), optional UPX compression, and whether to push a Docker image to Gitea and GitHub Container Registry.

Or build manually:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags "-X main.Verzija=1.0.0 -s -w" \
  -trimpath \
  -o ntech ./cmd/ntech
```

The result is a single static binary with no external dependencies.

---

## Environment Variables

The application reads environment variables on startup. In development, place them in `ntech.env` alongside the SQLite database file. In production/demo the program creates `ntech.env` automatically in the same directory as the database.

`ntech.env` is **never committed** to Git.

| Variable         | Default       | Description                                                       |
| ---------------- | ------------- | ----------------------------------------------------------------- |
| `NTECH_ENV`      | `development` | Mode: `development`, `production`, or `demo`                      |
| `NTECH_PORT`     | `8080`        | HTTP port                                                         |
| `NTECH_DB`       | `sqlite`      | Database type: `sqlite` or `postgres`                             |
| `NTECH_SQLITE`   | `ntech.db`    | Path to the SQLite file                                           |
| `NTECH_DSN`      | —             | PostgreSQL connection string                                      |
| `NTECH_SECRET`   | —             | Session signing key (min. 32 bytes); auto-generated if missing    |
| `NTECH_TOTP_KEY` | —             | AES-256 key for TOTP secret encryption; auto-generated if missing |
| `BE_ENABLED`     | `true`        | Enables the built-in card-emulator (fiscalization signing device) |
| `BE_PORT`        | `4567`        | TCP port for the built-in card-emulator                          |

`NTECH_SECRET` and `NTECH_TOTP_KEY` are generated automatically on the first run and saved to `ntech.env`. **Back this file up** — losing `NTECH_TOTP_KEY` invalidates all 2FA secrets stored in the database.

---

## Docker Deployment

Docker images are published to:
- `ghcr.io/dalibor31/ntech:latest`
- `git.vm-net.in.rs/dasko/ntech:latest`

### Production

```yaml
# docker-compose.yml
services:
  ntech:
    image: ghcr.io/dalibor31/ntech:latest
    restart: unless-stopped
    environment:
      NTECH_ENV: production
      NTECH_PORT: "8000"
      NTECH_SQLITE: /app/data/ntech.db
    volumes:
      - ./data:/app/data         # database + ntech.env (secrets)
      - ./uploads:/app/uploads   # uploaded images
      - ./logs:/app/logs         # structured + auth logs
      - ./backups:/app/backups   # automatic database backups
    ports:
      - "8000:8000"
```

On the **first start** the setup wizard runs and creates the first admin user. After that, `./data/ntech.env` contains the auto-generated secrets — **back it up**.

Place the app behind a reverse proxy (Caddy, nginx) that terminates HTTPS. Secure cookies require HTTPS.

Example Caddy config:

```
your.domain.com {
    reverse_proxy ntech:8000
}
```

### With Teron L-PFR Mock (Fiscalization Testing)

The `Fisk/` directory contains a Python mock server that simulates a [Teron](https://teron.rs) L-PFR fiscal device (port 4566). It implements the Teron HTTP API — invoice signing, PDV calculation, QR code generation, and per-type counters — without requiring real hardware or a certificate.

Use it alongside NTech for fiscalization development and testing:

```yaml
# docker-compose.yml
services:
  ntech:
    image: ghcr.io/dalibor31/ntech:latest
    container_name: ntech
    restart: unless-stopped
    ports:
      - "8000:8000"
    environment:
      NTECH_ENV: production
      NTECH_PORT: "8000"
      NTECH_SQLITE: /app/data/ntech.db
    volumes:
      - ./data:/app/data
      - ./uploads:/app/web/static/uploads
      - ./logs:/var/log/ntech
      - ./backups:/app/backups
    networks:
      - ntech-net

  teron-mock:
    image: ghcr.io/dalibor31/ntech-fisk:latest
    container_name: teron_mock
    restart: unless-stopped
    environment:
      - BE_HOST=ntech    # NTech service name on the shared network — required so the
      - BE_PORT=4567     # mock can read company data (name/PIB/address) from the card emulator
      - VERIFY_HOST=https://ntech.your-domain.com   # takes precedence over the "verify_host" setting
                                                     # in NTech's UI (the mock can't see ntech.db) —
                                                     # without it the QR links to sandbox.suf.purs.gov.rs
                                                     # and is sparse instead of encoding the full invoice
    volumes:
      - teron-data:/app/data
    networks:
      - ntech-net

volumes:
  teron-data:

networks:
  ntech-net:
```

The `teron-mock` service is reachable from `ntech` at `http://teron-mock:4566` over the internal Docker network — the port is not exposed to the host.

`BE_HOST`/`BE_PORT` must point to the NTech container's card emulator (`internal/be`, TCP port 4567). Without them, `teron-mock` defaults to `127.0.0.1:4567`, which inside its own container never reaches NTech — the mock then silently falls back to placeholder company data ("Test Company DOO", TIN `RS000000000`) instead of your real business profile.

To run as a standalone Docker container:

```yaml
# docker-compose.fisk.yml
services:
  teron-mock:
    image: ghcr.io/dalibor31/ntech-fisk:latest
    container_name: teron_mock
    restart: unless-stopped
    environment:
      - BE_HOST=ntech    # adjust to the NTech container's name/hostname on this network
      - BE_PORT=4567
      - VERIFY_HOST=https://ntech.your-domain.com
    ports:
      - "4566:4566"
    volumes:
      - teron-data:/app/data

volumes:
  teron-data:
```

```bash
docker compose -f docker-compose.fisk.yml up -d
```

To run the mock server locally (without Docker):

```bash
cd Fisk
pip install -r requirements.txt
python server.py
# or: ./start.sh
```

#### Teron Mock Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/status` | Device status and last invoice number |
| GET | `/api/attention` | Active alerts |
| POST | `/api/pin` | PIN verification (BE unlock) |
| GET | `/api/settings` | Device settings |
| POST | `/api/invoices` | Issue a fiscal invoice |
| POST | `/api/invoices/final` | Finalize an advance invoice |
| GET | `/api/invoices/last` | Last issued invoice |
| GET | `/api/invoices/:invoiceNumber` | Invoice by number |
| POST | `/api/invoices/search` | Search invoices |

Counters, signed receipts, QR codes, and JSON invoice data are persisted in `Fisk/data/`.

---

### Demo Mode

Demo mode runs a fully functional copy with a pre-created `Demo` / `Demo1234` admin account. Password and 2FA changes are blocked. Backup is limited to 2 copies.

```yaml
# docker-compose.yml (demo)
services:
  ntech-demo:
    image: ghcr.io/dalibor31/ntech:latest
    restart: unless-stopped
    environment:
      NTECH_ENV: demo
      NTECH_PORT: "8000"
      NTECH_SQLITE: /app/data/ntech.db
    volumes:
      - ./data:/app/data
      - ./uploads:/app/uploads
      - ./logs:/app/logs
      - ./backups:/app/backups
    ports:
      - "8000:8000"
```

Demo also requires HTTPS (Caddy or similar) because Secure cookies are enabled.

---

### Health Check

The app exposes an unauthenticated `GET /healthz` endpoint that pings the database and returns `200 OK` (or `503` if the database is unreachable). Use it for a Docker `HEALTHCHECK` or a reverse-proxy/orchestrator liveness probe:

```yaml
healthcheck:
  test: ["CMD", "wget", "-qO-", "http://localhost:8000/healthz"]
  interval: 30s
  timeout: 3s
  retries: 3
```

---

## Project Structure

```
ntech/
├── cmd/
│   └── ntech/          # entry point
├── internal/
│   ├── auth/           # login, sessions, fail2ban log
│   ├── config/         # settings, setup wizard
│   ├── db/             # database layer
│   │   └── sqlite/     # SQLite implementation
│   ├── handler/        # HTTP handlers
│   ├── middleware/      # CSRF, security headers, authentication
│   └── model/          # shared data types
├── web/
│   ├── static/         # CSS, JavaScript, images, logos
│   └── templates/      # HTML templates
├── migrations/         # SQL migrations (001_desc.sql, 002_desc.sql, ...)
├── logs/               # auth.log and other logs
├── backups/            # database backups
├── start.sh            # interactive build and Docker push script
├── Dockerfile
├── go.mod
└── go.sum
```

---

## Testing

The project has unit and integration tests (against a real SQLite database) covering crypto, RBAC, login flows, form validators, and reports.

```bash
go test ./...
```

Migrations are numbered SQL files (`migrations/NNN_description.sql`) applied in order at startup and tracked in a `migracije` table, so they run exactly once and are safe to ship inside the same binary.

---

## Security Notes

- Sessions are server-side (random token in an `HttpOnly`, `SameSite=Strict` cookie), not JWT — revocation is immediate (delete the row).
- CSRF tokens and the card-emulator PIN are compared in constant time (`crypto/subtle`).
- Brute-force locking applies to both the password step and the TOTP/backup-code step, keyed by client IP.
- `X-Real-IP` / `X-Forwarded-For` are only trusted when the connection itself comes from a loopback or private address (i.e. a reverse proxy on the same host/Docker network) — otherwise the raw connection IP is used, so the header can't be spoofed from the internet to bypass the lockout.
- TOTP secrets are encrypted at rest (AES-256-GCM); the key (`NTECH_TOTP_KEY`) is kept outside the database.
- This is a single-tenant, single-organization application by design — there is no cross-tenant data isolation to reason about.

See [`SECURITY.md`](SECURITY.md) for how to report a vulnerability.

---

## License

[MIT](LICENSE) © Dalibor Marković
