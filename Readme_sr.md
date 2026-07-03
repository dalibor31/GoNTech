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

## Kako radi

**Serversko renderovanje, bez build koraka.** Stranice se renderuju na serveru preko Go-ovog `html/template` (automatski escape, bez klijentskog šablonskog engine-a). [HTMX](https://htmx.org) menja delove stranice preko običnog HTTP-a za SPA-nalik navigaciju, a [Alpine.js](https://alpinejs.dev) pokriva manje delove klijentske interaktivnosti (live zbirovi, dinamički redovi u formama, autocomplete). Nema `npm`/webpack build korak — brauzer dobija čist HTML/CSS/JS, i jedno učitavanje stranice je dovoljno da se koristi cela aplikacija.

**Jedan binarni fajl, bez runtime zavisnosti.** Šabloni, statika (CSS/JS/slike) i SQL migracije su ugrađeni u kompajlirani binarni fajl preko `go:embed`. Deployment znači kopiranje jednog fajla (ili pokretanje jednog Docker image-a) — nema odvojenog build-a statike, nema fajlova šablona koje treba nositi uz izvršni fajl. U razvojnom modu isti kod čita direktno sa diska, pa su izmene šablona/CSS/JS-a odmah vidljive posle osvežavanja stranice, bez rebuild-a.

**Tok zahteva.** `chi` ruter provlači svaki zahtev kroz lanac middleware-a — bezbednosni headeri → CSRF (double-submit cookie) → provera sesije → provera uloge/dozvole — pre nego što stigne do handlera. Provera dozvola se izvršava na nivou rutera (da ruta ne bi slučajno ostala nezaštićena) i ponovo unutar handlera kao dodatni sloj zaštite. Handleri komuniciraju sa bazom kroz repository sloj (čist SQL, bez ORM-a) i prosleđuju obične Go strukture šablonima.

**Baza podataka.** SQLite preko čistog Go drajvera (`modernc.org/sqlite`, bez CGO-a) je podrazumevana i jedina zavisnost — jedan fajl, bez odvojenog servera baze koji treba pokretati ili bekapovati. Svako pokretanje primeni sve nove SQL migracione fajlove po redu i upiše ih u tabelu `migracije`, pa je nadogradnja samo "zameni binarni fajl i restartuj". Opcioni PostgreSQL backend (preko `pgx/v5`) je planiran za višekorisnička okruženja.

**Javne stranice za klijente.** Dva toka ne zahtevaju prijavu, samo jedinstven token koji se ne može pogoditi u URL-u: stranica statusa servisa (klijent prati napredak popravke i dobija QR kod direktno sa reversa) i stranica odobravanja predloga delova/usluga (klijent prihvata ili odbija procenu uz komentar). Oba su capability-bazirana — ko god ima link ima pristup tom jednom nalogu, ničemu drugom.

**Štampani dokumenti.** Radni nalog, predračun, otpremnica, revers i nalepnica uređaja su zasebne, samostalne HTML stranice stilizovane za A4 štampu (`@media print`), svaka sa dugmetom „Štampaj" koje otvara nativni dijalog za štampu u brauzeru — bez PDF biblioteke, bez headless-browser koraka za renderovanje. Dokumenti koji ne stanu na jednu stranicu sami se paginiraju na strani klijenta (merenjem visine sadržaja i ubacivanjem prekida strane sa brojevima strana).

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
- Magacin — artikli, kategorije, filtriranje, kritični nivoi zaliha, magacinska kartica po artiklu, veza sa dobavljačima, premeštanje artikala
- Barkod (EAN) po artiklu — pretraživ u magacinu; na ekranu prodaje, skeniranje barkoda (bilo kojim USB/Bluetooth skenerom koji „kuca" kod + Enter) automatski pronalazi artikal i dodaje ga u nalog
- Servisni nalozi:
  - Forma prijema, statusna traka, arhiva
  - Tok dijagnostike — opis kvara, napomene servisera, urađeno, cena dijagnostike
  - Delovi i radovi — ugrađeni artikli se skidaju sa lagera; predloženi artikli (ponuda klijentu)
  - Odobravanje predloga — klijent dobija javni link (QR kod) da prihvati ili odbije predlog sa komentarom
  - Praćenje statusa putem QR koda — nalepnica na uređaju i svaki štampani dokument nose QR kod koji klijent skenira telefonom; otvara se mobilno optimizovana stranica (bez prijave, bez aplikacije) sa trenutnim statusom popravke, kojoj klijent može ponovo da pristupi u bilo kom trenutku istim linkom/kodom
  - Dokumenti — radni nalog, predračun, otpremnica, revers, nalepnica za uređaj (QR + Code128 barkod)
  - Preuzimanje sa naplatom — način plaćanja i iznos avansa
  - Garancija, predviđen datum završetka, serviser, napomena klijentu
- Prodajni nalozi — stavke, obračun, priznanica sa podacima firme i klijenta
- Cenovnik usluga — šifarnik usluga za obračun u servisnim nalozima
- Troškovi — evidencija troškova po kategoriji i iznosu
- Nabavke — evidencija nabavki od dobavljača
- Kalkulacija prodajne cene pri nabavci — marža (globalna, po kategoriji i po artiklu), zavisni troškovi (carina, prevoz...) sa raspodelom na stavke, dvosmerni izračun marža↔prodajna; poštuje status PDV obveznika
- Nivelacija — promena prodajne cene uz trag (istorija promena: stara→nova, razlog, izvor, korisnik)
- Profil firme i moduli — funkcije se uključuju prema tipu firme i statusu PDV obveznika
- PDV evidencija (KIR/KPR) — knjige izdatih i primljenih računa, automatsko punjenje iz prodaje i nabavke
- PDV obračun za period + mapiranje na obrazac PP-PDV; uvoz robe (JCI) se vodi u poljima 006/106
- Šifarnik PDV stopa
- **Fiskalizacija (ESIR/L-PFR)** — pun Go klijent za Teron API fiskalnog uređaja: test konekcije, izdavanje računa (prodaja/servis, uključujući avanse i refund pri stornu), dnevni pazar, zaključenje fiskalnog dana, PDF fiskalni izveštaji, QR verifikacija računa (javna `/v/` stranica), automatski retry pri neuspešnoj fiskalizaciji sa vidljivim statusom greške, i panel za status/reset kartica-emulatora (komunicira sa uređajem za potpisivanje preko TCP-a). Trenutno provereno protiv Teron mock servera (`Fisk/`); nije još testirano na pravom sertifikovanom uređaju.
- Klijenti i dobavljači — baza kontakata
- Podsetnici — evidencija sa rokom
- Izveštaji — pregled prihoda, stanje magacina, vrednost zaliha, prometni list, popis (inventura)
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

### U toku

- **Fiskalizacija na pravom uređaju** — ceo tok (izdavanje, refund, dnevni pazar, zaključenje, izveštaji) je implementiran i proveren protiv Teron mock servera; testiranje na pravom sertifikovanom L-PFR uređaju još nije urađeno.

### Planirano

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
| [HTMX](https://htmx.org)                                                             | dinamički HTML preko HTTP-a     |
| [Alpine.js](https://alpinejs.dev)                                                    | UI logika na strani klijenta    |
| [SQLite](https://sqlite.org) + [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) | glavna baza (čisti Go, bez CGO) |
| [PostgreSQL](https://www.postgresql.org) + [pgx/v5](https://github.com/jackc/pgx)    | opciona baza za produkciju      |

---

## Pokretanje

### Zahtevi

- Go 1.26 ili noviji
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
| `BE_ENABLED`     | `true`        | Uključuje ugrađeni kartica-emulator (uređaj za potpisivanje pri fiskalizaciji) |
| `BE_PORT`        | `4567`        | TCP port ugrađenog kartica-emulatora                               |

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

### Sa Teron L-PFR mokom (testiranje fiskalizacije)

Folder `Fisk/` sadrži Python mock server koji simulira [Teron](https://teron.rs) L-PFR fiskalni uređaj (port 4566). Implementira Teron HTTP API — potpisivanje računa, PDV obračun, generisanje QR koda i brojače po tipu računa — bez potrebe za pravim hardverom ili sertifikatom.

Koristi se uz NTech za razvoj i testiranje fiskalizacije:

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

Servis `teron-mock` je dostupan iz `ntech` kontejnera na adresi `http://teron-mock:4566` preko interne Docker mreže — port nije izložen spolja.

Za pokretanje kao samostalni Docker kontejner:

```yaml
# docker-compose.fisk.yml
services:
  teron-mock:
    image: ghcr.io/dalibor31/ntech-fisk:latest
    container_name: teron_mock
    restart: unless-stopped
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

Za lokalno pokretanje moka (bez Dockera):

```bash
cd Fisk
pip install -r requirements.txt
python server.py
# ili: ./start.sh
```

#### Teron Mock endpointi

| Metod | Putanja | Opis |
|-------|---------|------|
| GET | `/api/status` | Status uređaja i poslednji broj računa |
| GET | `/api/attention` | Aktivna upozorenja |
| POST | `/api/pin` | Verifikacija PIN-a (otključavanje BE) |
| GET | `/api/settings` | Podešavanja uređaja |
| POST | `/api/invoices` | Izdavanje fiskalnog računa |
| POST | `/api/invoices/final` | Finalizacija avansnog računa |
| GET | `/api/invoices/last` | Poslednji izdati račun |
| GET | `/api/invoices/:invoiceNumber` | Račun po broju |
| POST | `/api/invoices/search` | Pretraga računa |

Brojači, potpisane priznanice, QR kodovi i JSON podaci o računima čuvaju se u `Fisk/data/`.

---

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

### Health check

Aplikacija izlaže neautentifikovan `GET /healthz` endpoint koji proverava dostupnost baze i vraća `200 OK` (ili `503` ako baza nije dostupna). Koristiti za Docker `HEALTHCHECK` ili proveru živosti u reverse proxy-ju/orkestratoru:

```yaml
healthcheck:
  test: ["CMD", "wget", "-qO-", "http://localhost:8000/healthz"]
  interval: 30s
  timeout: 3s
  retries: 3
```

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

---

## Testiranje

Projekat ima jedinične i integracione testove (nad pravom SQLite bazom) koji pokrivaju kripto funkcije, RBAC, tokove prijave, validatore formi i izveštaje.

```bash
go test ./...
```

Migracije su numerisani SQL fajlovi (`migrations/NNN_opis.sql`) koji se primenjuju redom pri pokretanju i prate se u tabeli `migracije` — izvršavaju se tačno jednom i bezbedno je da putuju unutar istog binarnog fajla.

---

## Bezbednosne napomene

- Sesije se čuvaju na serveru (nasumičan token u `HttpOnly`, `SameSite=Strict` kolačiću), ne JWT — opoziv je trenutan (brisanje reda).
- CSRF token i PIN kartica-emulatora se porede konstantno-vremenski (`crypto/subtle`).
- Bruteforce zaključavanje važi i za korak lozinke i za korak TOTP/rezervnog koda, po IP adresi klijenta.
- `X-Real-IP` / `X-Forwarded-For` se veruje samo kada sama konekcija dolazi sa loopback ili privatne adrese (tj. reverse proxy na istom hostu/Docker mreži) — inače se koristi sirovi IP konekcije, pa se header ne može lažirati sa interneta radi zaobilaženja zaključavanja.
- TOTP tajne su šifrovane u mirovanju (AES-256-GCM); ključ (`NTECH_TOTP_KEY`) se čuva van baze.
- Ovo je namerno jednokorisnička/jednoorganizaciona aplikacija — nema izolacije podataka između više firmi (multi-tenant) o kojoj treba brinuti.

Pogledaj [`SECURITY.md`](SECURITY.md) za način prijave bezbednosnog propusta.

---

## Licenca

[MIT](LICENSE) © Dalibor Marković
