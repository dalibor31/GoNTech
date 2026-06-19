# NTech

[🇬🇧 English version](Readme.md)

<img width="1440" height="754" alt="image" src="https://github.com/user-attachments/assets/05ade210-6c54-41d8-8006-707c3077944d" />

![Go Version](https://img.shields.io/badge/go-1.26-blue)
![License](https://img.shields.io/badge/license-MIT-green)

Poslovna aplikacija za upravljanje servisom računara, magacinom delova i prodajom. Napravljena u Go-u, radi u brauzeru, ne zahteva internet vezu ni eksterne servise.

> ⚠️ Projekat je u aktivnom razvoju. Nije spreman za produkcijsku upotrebu.

**Demo:** [https://demo.vm-net.in.rs](https://demo.vm-net.in.rs) — prijava sa `Demo` / `Demo1234` (uloga: admin; promena lozinke i 2FA su u demo modu onemogućeni).

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
- Dvofaktorska autentifikacija (TOTP) — aktivacija sa QR kodom; tajna šifrovana u bazi (AES-256-GCM, ključ van baze)
- Rezervni (jednokratni) kodovi za 2FA — generišu se pri aktivaciji, čuvaju kao bcrypt heš; alternativa TOTP-u pri prijavi
- Bruteforce zaštita — IP zaključavanje nakon 5 neuspelih pokušaja u 15 minuta
- CSRF zaštita — double-submit cookie pattern, automatska injekcija tokena u sve forme
- Bezbednosni HTTP headeri (CSP, X-Frame-Options, Referrer-Policy, nosniff...)
- Evidencija pokušaja prijave — istorija po korisniku, IP, razlog, datum
- Korisnici i uloge — admin panel, upravljanje korisnicima
- Magacin — artikli, kategorije, filtriranje, kritični nivoi zaliha
- Servisni nalozi — prijem, statusna traka, troškovi, priznanica
- Prodajni nalozi — stavke, obračun, priznanica sa podacima firme i klijenta
- Nabavke — evidencija nabavki od dobavljača
- Kalkulacija prodajne cene pri nabavci — marža (globalna, po kategoriji i po artiklu), zavisni troškovi (carina, prevoz...) sa raspodelom na stavke, dvosmerni izračun marža↔prodajna; poštuje status PDV obveznika
- Nivelacija — promena prodajne cene uz trag (istorija promena: stara→nova, razlog, izvor, korisnik)
- Profil firme i moduli — funkcije se uključuju prema tipu firme i statusu PDV obveznika
- PDV evidencija (KIR/KPR) — knjige izdatih i primljenih računa, automatsko punjenje iz prodaje i nabavke
- PDV obračun za period + mapiranje na obrazac PP-PDV; uvoz robe (JCI) se vodi u poljima 006/106
- Šifarnik PDV stopa
- Klijenti i dobavljači — baza kontakata
- Podsetnici — evidencija sa rokom
- Izveštaji — pregled prihoda, stanje magacina
- Podešavanja — naziv, adresa, PIB, logo firme; promena teme
- Pozadinske slike — login stranica i aplikacija, sa zamućenjem, providnošću i glass efektom
- Lična tema i pozadina — svaki korisnik može svoju temu i pozadinsku sliku
- Matrica dozvola (RBAC) — admin panel za dozvole po ulogama; provera se sprovodi na nivou ruta (i mutirajućih i pregleda) i u handlerima
- Flash poruke — jednokratne povratne informacije nakon akcije
- Automatski backup SQLite baze — sa podešavanjem broja čuvanih kopija; vraćanje baze iz kopije (bezbedno, bez prekida rada)
- Grafikoni — mesečni prihod na izveštajima (Chart.js)
- Strukturisano logovanje — `log/slog` (JSON u produkciji, tekst u razvoju); zaseban auth log u fail2ban formatu
- Automatski testovi — jedinični i integracioni nad SQLite bazom (kripto, RBAC, tokovi prijave, validatori forme, izveštaji)
- **Demo mod** (`NTECH_ENV=demo`) — automatski kreiran demo korisnik, pre-popunjeni login, ograničen bekap, blokirana promena lozinke i 2FA

### Planirano

- Fiskalizacija (ESIR/PFR)
- KPO knjiga i dvojno knjigovodstvo (opciono, kasnija faza)
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

- Go 1.24 ili noviji
- Git

### Koraci

```bash
# 1. Kloniranje repozitorijuma
git clone <url-repozitorijuma>
cd GoNtech

# 2. Pokretanje u razvojnom modu (čita fajlove sa diska, ne zahteva HTTPS)
go run ./cmd/ntech
```

Program se otvara na `http://localhost:8080`. Pri prvom pokretanju automatski se pokreće setup wizard.

### Produkcioni build

Koristi interaktivnu skriptu:

```bash
./start.sh
```

Skripta pita za verziju, okruženje (production/development), platformu (Linux/Windows/obe), opcionalnu UPX kompresiju i da li da gurne Docker image na Gitea i GitHub Container Registry.

Ili ručno:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags "-X main.Verzija=1.0.0 -s -w" \
  -trimpath \
  -o ntech ./cmd/ntech
```

Rezultat je jedan statički binarni fajl bez zavisnosti.

---

## Promenljive okruženja

Program čita promenljive okruženja pri pokretanju. U razvojnom modu staviti ih u `ntech.env` pored SQLite baze. U production/demo modu program sam kreira `ntech.env` u istom folderu gde je baza.

Fajl `ntech.env` se **ne commituje** u Git.

| Promenljiva      | Podrazumevano | Opis                                                               |
| ---------------- | ------------- | ------------------------------------------------------------------ |
| `NTECH_ENV`      | `development` | Mod: `development`, `production` ili `demo`                        |
| `NTECH_PORT`     | `8080`        | HTTP port                                                          |
| `NTECH_DB`       | `sqlite`      | Tip baze: `sqlite` ili `postgres`                                  |
| `NTECH_SQLITE`   | `ntech.db`    | Putanja do SQLite fajla                                            |
| `NTECH_DSN`      | —             | PostgreSQL connection string                                       |
| `NTECH_SECRET`   | —             | Ključ za potpisivanje sesija (min. 32 bajta); auto-generiše se     |
| `NTECH_TOTP_KEY` | —             | AES-256 ključ za šifrovanje TOTP tajni; auto-generiše se          |

`NTECH_SECRET` i `NTECH_TOTP_KEY` se automatski generišu pri prvom pokretanju i upisuju u `ntech.env`. **Sačuvaj backup ovog fajla** — gubitak `NTECH_TOTP_KEY` onemogućuje prijavu svim korisnicima koji imaju 2FA.

---

## Docker deployment

Docker image je dostupan na:
- `ghcr.io/dalibor31/ntech:latest`
- `git.vm-net.in.rs/dasko/ntech:latest`

### Produkcija

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
      - ./data:/app/data         # baza + ntech.env (tajne)
      - ./uploads:/app/uploads   # uploadovane slike
      - ./logs:/app/logs         # strukturisani + auth log
      - ./backups:/app/backups   # automatski bekap baze
    ports:
      - "8000:8000"
```

Pri **prvom pokretanju** pokreće se setup wizard za kreiranje prvog admin korisnika. Nakon toga, `./data/ntech.env` sadrži auto-generisane tajne — **sačuvaj backup**.

Stavi program iza reverznog proksija (Caddy, nginx) koji terminira HTTPS. Secure kolačići zahtevaju HTTPS.

Primer Caddy konfiguracije:

```
tvoj.domen.com {
    reverse_proxy ntech:8000
}
```

### Demo mod

Demo mod pokreće potpuno funkcionalnu kopiju sa pre-kreiranim nalogom `Demo` / `Demo1234` (admin). Promena lozinke i 2FA su blokirani. Bekap je ograničen na 2 kopije.

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

Demo takođe zahteva HTTPS (Caddy ili slično) jer su Secure kolačići uključeni.

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
├── start.sh            # interaktivna skripta za build i Docker push
├── Dockerfile
├── go.mod
└── go.sum
```
