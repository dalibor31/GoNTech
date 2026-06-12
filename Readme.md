# NTech

[🇷🇸 Srpska verzija](Readme_sr.md)

![Go Version](https://img.shields.io/badge/go-1.26-blue)
![License](https://img.shields.io/badge/license-MIT-green)

<img width="1440" height="754" alt="image" src="https://github.com/user-attachments/assets/2ea0746f-2f8e-46f7-9551-06e49c0e61ed" />

A business application for computer repair shop management, parts inventory tracking, and sales. Built with Go, it runs in the browser and requires no internet connection or external services.

> ⚠️ The project is under active development. It is not ready for production use.

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
- Brute-force protection — IP locking after 5 failed attempts within 15 minutes
- CSRF protection — double-submit cookie pattern, automatic token injection into all forms
- Security HTTP headers (CSP, X-Frame-Options, Referrer-Policy, nosniff...)
- Login attempt logging — history by user, IP, reason, date
- Users and roles — admin panel, user management
- Inventory — items, categories, filtering, critical stock levels
- Service orders — intake, status bar, costs, receipt
- Sales orders — items, calculation, receipt with company and client details
- Procurement — records of purchases from suppliers
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

### Planned

- Fiscalization and VAT calculation (specification in Project.md)
- PostgreSQL support (for multi-user environments)
- WebAuthn / Passkey login (database schema is already prepared)
- Backup (one-time) codes for 2FA
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

# 2. Copy the configuration file
cp ntech.env.example ntech.env
# Open ntech.env and set the values (see the table below)

# 3. Load environment variables and run in the development environment
export $(grep -v '^#' ntech.env | xargs)
go run ./cmd/ntech
```
