# NTech

![Go Version](https://img.shields.io/badge/go-1.26-blue)
![License](https://img.shields.io/badge/license-MIT-green)

Poslovna aplikacija za upravljanje servisom računara, magacinom delova i prodajom. Napravljena u Go-u, radi u brauzeru, ne zahteva internet vezu ni eksterne servise.

> ⚠️ Projekat je u aktivnom razvoju. Nije spreman za produkcijsku upotrebu.

---

## O projektu

NTech je interna aplikacija napravljena za konkretnog korisnika — servis računara koji pored popravki vodi i magacin delova, prodaju komponenti i gotovih konfiguracija, te evidenciju klijenata i dobavljača.

Cilj je jednostavan: sve što servis treba da prati nalazi se na jednom mestu, bez oslanjanja na tabele u Excelu ili papirnu evidenciju.

---

## Funkcionalnosti

### Implementirano

- Inicijalno podešavanje pri prvom pokretanju (setup wizard)
- Sistem migracija baze podataka
- Osnovna struktura baze: artikli, kategorije, klijenti, dobavljači, servisni nalozi, prodajni nalozi, podsetnici

### U razvoju

- Korisničko sučelje — sidebar navigacija, sistem tema, dashboard
- Magacin — praćenje stanja delova, lokacija, količina
- Servisni nalozi — prijem, dijagnostika, statusna traka, cene
- Prodajni nalozi — stavke, obračun, evidencija kupaca
- Klijenti i dobavljači — baza kontakata, istorija

### Planirano

- Prijava korisnika — sesije, zaključavanje naloga
- Dvofaktorska autentifikacija (TOTP)
- Izveštaji — prodaja, stanje magacina, prihodi
- Podrška za PostgreSQL (za višekorisničko okruženje)
- Obaveštenja (e-pošta / WhatsApp) — odloženo za kasniju fazu
- Skeniranje barkodova putem kamere — odloženo za kasniju fazu

---

## Tehnologije

| Tehnologija                                                                          | Uloga                            |
| ------------------------------------------------------------------------------------ | -------------------------------- |
| [Go](https://go.dev)                                                                 | backend jezik                    |
| [chi](https://github.com/go-chi/chi)                                                 | HTTP ruter                       |
| [html/template](https://pkg.go.dev/html/template)                                    | serverski šabloni                |
| [HTMX](https://htmx.org)                                                             | interaktivnost bez build procesa |
| [TailwindCSS](https://tailwindcss.com)                                               | stilizovanje                     |
| [Alpine.js](https://alpinejs.dev)                                                    | UI logika na strani klijenta     |
| [SQLite](https://sqlite.org) + [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) | glavna baza (čisti Go, bez CGO)  |
| [PostgreSQL](https://www.postgresql.org) + [pgx/v5](https://github.com/jackc/pgx)    | opciona baza za produkciju       |

---

## Pokretanje

### Zahtevi

- Go 1.22 ili noviji
- Git

### Koraci

```bash
# 1. Kloniranje repozitorijuma
git clone <url-repozitorijuma>
cd GoNtech

# 2. Kopiranje i popunjavanje .env fajla
cp .env.example .env
# Otvori .env i postavi vrednosti (videti tabelu ispod)

# 3. Pokretanje u razvojnom okruženju
go run ./cmd/ntech
```

Program se otvara na `http://localhost:8080`.

Pri prvom pokretanju automatski se pokreće setup wizard.

### Produkcioni build

```bash
CGO_ENABLED=0 go build -o ntech ./cmd/ntech
./ntech
```

Rezultat je jedan statički binarni fajl bez zavisnosti.

---

## Promenljive okruženja

Kopirati `.env.example` u `.env` i popuniti vrednosti. Fajl `.env` se **ne commituje** u Git.

| Promenljiva    | Podrazumevano | Opis                                         |
| -------------- | ------------- | -------------------------------------------- |
| `NTECH_ENV`    | `development` | Okruženje: `development` ili `production`    |
| `NTECH_PORT`   | `8080`        | HTTP port                                    |
| `NTECH_DB`     | `sqlite`      | Tip baze: `sqlite` ili `postgres`            |
| `NTECH_SQLITE` | `ntech.db`    | Putanja do SQLite fajla                      |
| `NTECH_DSN`    | —             | PostgreSQL connection string                 |
| `NTECH_SECRET` | —             | Ključ za potpisivanje sesija (min. 32 bajta) |

---

## Struktura projekta

```
ntech/
├── cmd/
│   └── ntech/          # ulazna tačka programa
├── internal/
│   ├── auth/           # prijava, sesije, 2FA
│   ├── config/         # podešavanja, setup wizard
│   ├── db/             # sloj baze podataka
│   │   ├── sqlite/     # SQLite implementacija
│   │   └── postgres/   # PostgreSQL implementacija
│   ├── handler/        # HTTP handleri
│   ├── middleware/      # autentifikacija, logovanje, ograničavanje zahteva
│   └── model/          # zajednički tipovi podataka
├── web/
│   ├── static/         # CSS, JavaScript, slike
│   └── templates/      # HTML šabloni
├── migrations/         # SQL migracije (001_opis.sql, 002_opis.sql, ...)
├── .env.example        # primer konfiguracije
├── go.mod
└── go.sum
```
