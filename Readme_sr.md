# NTech

[🇬🇧 English version](Readme.md)

![Go Version](https://img.shields.io/badge/go-1.24-blue)
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
- Korisnički interfejs — sidebar navigacija, sistem tema (tamna/svetla), dashboard sa statistikama
- Prijava korisnika — sesije na serveru, zaključavanje naloga
- Dvofaktorska autentifikacija (TOTP) — aktivacija sa QR kodom, rezervni kodovi
- Bruteforce zaštita — IP zaključavanje nakon 5 neuspelih pokušaja u 15 minuta
- CSRF zaštita — double-submit cookie pattern, automatska injekcija tokena u sve forme
- Bezbednosni HTTP headeri (CSP, X-Frame-Options, Referrer-Policy, nosniff...)
- Evidencija pokušaja prijave — istorija po korisniku, IP, razlog, datum
- Korisnici i uloge — admin panel, upravljanje korisnicima
- Magacin — artikli, kategorije, filtriranje, kritični nivoi zaliha
- Servisni nalozi — prijem, statusna traka, troškovi, priznanica
- Prodajni nalozi — stavke, obračun, priznanica sa podacima firme i klijenta
- Nabavke — evidencija nabavki od dobavljača
- Klijenti i dobavljači — baza kontakata
- Podsetnici — evidencija sa rokom
- Izveštaji — pregled prihoda, stanje magacina
- Podešavanja — naziv, adresa, PIB, logo firme; promena teme

### Planirano

- Podrška za PostgreSQL (za višekorisničko okruženje)
- WebAuthn / Passkey prijava (šema baze je pripremljena)
- Obaveštenja (e-pošta / WhatsApp) — odloženo za kasniju fazu
- Skeniranje barkodova putem kamere — odloženo za kasniju fazu

---

## Tehnologije

| Tehnologija                                                                          | Uloga                           |
| ------------------------------------------------------------------------------------ | ------------------------------- |
| [Go](https://go.dev)                                                                 | backend jezik                   |
| [chi](https://github.com/go-chi/chi)                                                 | HTTP ruter                      |
| [html/template](https://pkg.go.dev/html/template)                                    | serverski šabloni               |
| [Alpine.js](https://alpinejs.dev)                                                    | UI logika na strani klijenta    |
| [SQLite](https://sqlite.org) + [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) | glavna baza (čisti Go, bez CGO) |
| [PostgreSQL](https://www.postgresql.org) + [pgx/v5](https://github.com/jackc/pgx)    | opciona baza za produkciju      |

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

# 2. Kopiranje konfiguracionog fajla
cp ntech.env.example ntech.env
# Otvori ntech.env i postavi vrednosti (videti tabelu ispod)

# 3. Učitavanje promenljivih i pokretanje u razvojnom okruženju
export $(grep -v '^#' ntech.env | xargs)
go run ./cmd/ntech
```

Program se otvara na `http://localhost:8080` (ili na portu definisanom u `ntech.env`).

Pri prvom pokretanju automatski se pokreće setup wizard.

### Produkcioni build

```bash
# Pomoću build.sh skripte (prima opcioni argument verzije)
./build.sh 1.0.0

# Ili ručno
CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build \
  -ldflags "-X main.Verzija=1.0.0 -s -w" \
  -o ntech ./cmd/ntech
./ntech
```

Rezultat je jedan statički binarni fajl bez zavisnosti.

---

## Promenljive okruženja

Kopirati `ntech.env.example` u `ntech.env` i popuniti vrednosti. Fajl `ntech.env` se **ne commituje** u Git.

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
│   ├── auth/           # prijava, sesije, fail2ban log
│   ├── config/         # podešavanja, setup wizard
│   ├── db/             # sloj baze podataka
│   │   └── sqlite/     # SQLite implementacija
│   ├── handler/        # HTTP handleri
│   ├── middleware/      # CSRF, bezbednost headeri, autentifikacija
│   └── model/          # zajednički tipovi podataka
├── web/
│   ├── static/         # CSS, JavaScript, slike, logotipi
│   └── templates/      # HTML šabloni
├── migrations/         # SQL migracije (001_opis.sql, 002_opis.sql, ...)
├── logs/               # auth.log i ostali logovi
├── backups/            # rezervne kopije baze
├── build.sh            # skripta za produkcioni build
├── ntech.env           # lokalna konfiguracija (ne commituje se)
├── go.mod
└── go.sum
```
