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
- Inventory — items, categories, filtering, critical stock levels
- Service orders — intake, status bar, costs, receipt
- Sales orders — items, calculation, receipt with company and client details
- Procurement — records of purchases from suppliers
- Sales price calculation on procurement — markup (global, per category, per item), landed costs (customs, shipping...) allocated across items, two-way markup↔price computation; respects VAT-payer status
- Price revaluation (nivelacija) — sales price changes with an audit trail (old→new, reason, source, user)
- Company profile and modules — features toggle based on company type and VAT-payer status
- VAT records (KIR/KPR) — books of issued and received invoices, auto-filled from sales and procurement
- VAT calculation per period + mapping to the PP-PDV form; imports (customs declaration) tracked in fields 006/106
- VAT rate code list
- Clients and suppliers — contact database
- Reminders — records with deadlines
- Reports — revenue overview, inventory status
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

- **Fiscalization (ESIR/PFR)** — Teron L-PFR mock server included in `Fisk/`; Go client integration planned

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
| [Alpine.js](https://alpinejs.dev)                                                    | client-side UI logic            |
| [SQLite](https://sqlite.org) + [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) | main database (pure Go, no CGO) |
| [PostgreSQL](https://www.postgresql.org) + [pgx/v5](https://github.com/jackc/pgx)    | optional production database    |

---

## Running the Application

### Requirements

- Go 1.24 or newer
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
