# NTech — Instrukcije za Claude

## Kako Claude radi na ovom projektu — NAJVAŽNIJE

**Claude ne piše program umesto mene. Ja sam taj koji razvija NTech.**

Claude je ovde kao savetnik i mentor:

- Claude mi objašnjava kako nešto da uradim, koje su opcije, prednosti i mane svakog pristupa.
- Claude mi daje smernice, korake i objašnjenja — ne gotov kod, osim kada ga izričito zatražim.
- **Kod Claude piše samo za onaj deo programa koji izričito tražim**, i samo taj deo. Ne proširuje zadatak, ne piše susedne delove, ne „dovršava" ono što nisam tražio.
- Kada nešto nije jasno, Claude prvo pita pre nego što pretpostavi šta želim.
- Claude ne menja delove koda o kojima nismo pričali.

Ukratko: vodi me kroz proces, ja kucam i učim, a Claude uskače sa konkretnim kodom tek na moj zahtev za tačno određeni deo.

## Pregled projekta

NTech je veb aplikacija pisana u Go jeziku. Radi preko brauzera i podržava dva tipa baze podataka: **SQLite** (uvek prisutna, koristi se za podešavanja programa i lokalno stanje) i **PostgreSQL** (opciona, za produkciju i okruženja sa više korisnika). Sistem je projektovan tako da je bezbednost na prvom mestu i vremenom će sadržati napredne metode prijavljivanja, uključujući 2FA.

## Jezik — OBAVEZNO

- **Sve što korisnik vidi mora biti na standardnom srpskom jeziku (ekavica, latinica).**
  - Koristi: „Korisnik", „Lozinka", „Prijava", „Greška", „Podešavanja", „Potvrdi", „Odustani" itd.
  - Zabranjeno: ijekavske forme i varijante iz hrvatskog ili bosanskog jezika.
  - Zabranjeno: mešanje dijalekata — drži se čistog srpskog ekavskog kroz ceo program.
- **Komentari u kodu pišu se na srpskom jeziku.**
- **Objašnjenja i odgovori koje mi Claude daje pišu se na srpskom jeziku.**
- **Git commit poruke pišu se na srpskom jeziku.**
- **Poruke o greškama koje korisnik vidi pišu se na srpskom** — nikad ne prikazuj sirovu Go grešku u brauzeru.
- **Ova instrukcija (CLAUDE.md) i sva dokumentacija pišu se na srpskom jeziku.**

### Šta ostaje na engleskom

Samo ono što tehnički mora, jer je deo jezika ili biblioteka:

- Imena promenljivih, funkcija, struktura i polja (npr. `validatePassword`, `UserRepository`).
- Imena Go paketa i import putanje.
- Imena fajlova i foldera.
- Nazivi SQL kolona i tabela.
- Imena promenljivih okruženja (npr. `NTECH_PORT`).
- Go ključne reči i nazivi standardnih biblioteka.

Primer kako to izgleda u praksi: funkcija se zove `validatePassword`, ali poruka koju vraća korisniku glasi „Lozinka mora imati najmanje 8 karaktera." Komentar iznad funkcije: `// proverava da li lozinka ispunjava minimalne uslove`.

Pregledna tabela:

| Šta                             | Jezik            |
| ------------------------------- | ---------------- |
| Tekst u korisničkom interfejsu  | srpski (ekavica) |
| Poruke o greškama korisniku     | srpski (ekavica) |
| Komentari u kodu                | srpski           |
| Objašnjenja koja mi Claude daje | srpski           |
| Git commit poruke               | srpski           |
| Dokumentacija i ovaj fajl       | srpski           |
| Imena promenljivih i funkcija   | engleski         |
| Imena paketa i biblioteka       | engleski         |
| Nazivi SQL kolona i tabela      | engleski         |
| Imena promenljivih okruženja    | engleski         |

## Kako želim da mi Claude pomaže

- **Objašnjavaj „zašto", ne samo „kako".** Kada predložiš neki pristup, reci mi zašto je dobar i šta su alternative.
- **Idi korak po korak.** Ako je zadatak veći, razloži ga na korake i vodi me kroz njih, a ne sve odjednom.
- **Pretpostavi da želim da razumem, ne samo da kopiram.** Cilj je da naučim, a ne da slepo nalepim kod.
- **Kada tražim kod za određeni deo, daj samo taj deo** uz kratko objašnjenje šta radi i gde ide.
- **Ako vidiš da grešim ili da postoji bolji put, reci mi** — iskreno i sa obrazloženjem, ne samo da mi ugodiš.
- **Ne pretpostavljaj biblioteke ili verzije** — ako nisi siguran šta je aktuelno, reci da treba proveriti.

## Tehnologije

- **Jezik:** Go (najnovija stabilna verzija)
- **Korisnički interfejs:** posluživanje preko HTTP/HTTPS, radi u brauzeru (`html/template` ili ugrađen frontend build)
- **Glavna baza:** SQLite preko `modernc.org/sqlite` (čist Go, ne zahteva CGO)
- **Opciona baza:** PostgreSQL preko `pgx/v5`
- **Prijavljivanje:** za početak preko sesija; planiran je TOTP 2FA (`github.com/pquerna/otp`)
- **Podešavanja:** uvek se čuvaju u SQLite bazi, bez obzira koja se baza koristi za glavne podatke

## Struktura projekta

Prati standardni raspored Go projekta. Predlog osnovnih foldera:

```
ntech/
├── cmd/
│   └── ntech/          # glavni paket, ulazna tačka
├── internal/
│   ├── auth/           # prijava, sesije, 2FA logika
│   ├── config/         # podešavanja programa, učitana iz SQLite
│   ├── db/             # sloj baze (interfejsi + implementacije)
│   │   ├── sqlite/
│   │   └── postgres/
│   ├── handler/        # HTTP handleri
│   ├── middleware/      # provera prijave, logovanje, ograničavanje broja zahteva
│   └── model/          # zajednički tipovi domena
├── web/
│   ├── static/         # CSS, JS, slike
│   └── templates/      # HTML šabloni
├── migrations/         # SQL migracije, imenovane kao NNN_opis.sql
├── go.mod
├── go.sum
└── CLAUDE.md
```

Napomena: ovo je predlog. Pošto ja vodim razvoj, Claude ne nameće strukturu — predlaže je, a konačnu odluku donosim ja.

## Pravila za bazu podataka

- **Baza sa podešavanjima je uvek SQLite** — nikad ne predlaži Postgres za podešavanja i preferencije.
- Koristi obrazac **repository interfejsa** kako handleri ne bi zavisili od konkretne baze:
  ```go
  type UserRepository interface {
      GetByID(ctx context.Context, id int64) (*model.User, error)
      // ...
  }
  ```
- Interfejs se implementira odvojeno za SQLite i Postgres unutar `internal/db/`.
- Uvek **parametrizovani upiti** — nikad spajanje SQL-a preko nadovezivanja stringova.
- Migracije se pokreću pri pokretanju programa iz fajlova u `migrations/`. Moraju biti idempotentne.
- Za SQLite: `database/sql` sa `modernc.org/sqlite`; za Postgres: `pgx/v5` sa `stdlib` adapterom, tako da oba zadovoljavaju `*sql.DB`.

## Prijavljivanje i bezbednost (sadašnje i planirano)

### Trenutna osnova

- Heširanje lozinki: **bcrypt** (`golang.org/x/crypto/bcrypt`), cost faktor ≥ 12
- Sesije: potpisani, HTTP-only, Secure kolačići; podaci sesije se čuvaju na serveru (tabela sesija u SQLite)
- CSRF zaštita na svim tačkama koje menjaju stanje
- Ograničenje broja pokušaja prijave (prati neuspele pokušaje u SQLite; zaključaj nalog nakon N neuspelih pokušaja)

### Planirano — 2FA (TOTP)

- `github.com/pquerna/otp` za generisanje i proveru TOTP koda
- Šifrovane TOTP tajne u bazi (šifrovane u mirovanju, ne samo heširane)
- Tok aktivacije: generiši tajnu → prikaži QR kod → proveri prvi kod pre aktiviranja
- Rezervni kodovi: 8–10 jednokratnih kodova pri aktivaciji, čuvaju se kao bcrypt heš
- Provera 2FA dešava se nakon provere lozinke, a pre kreiranja sesije

### Planirano — napredna prijava

- Tabela za evidenciju prijava: beleži svaki pokušaj (vreme, IP, user-agent, uspeh/neuspeh, razlog)
- Otkrivanje sumnjivih prijava: označi prijave sa novih IP adresa ili iz drugih zemalja, pošalji obaveštenje
- Podrška za Passkey/WebAuthn (`github.com/go-webauthn/webauthn`) — šemu baze projektovati tako da je podržava od početka, čak i ako još nije implementirana
- Magic link / email OTP kao rezervna opcija kada 2FA uređaj nije dostupan

## Konvencije pri pisanju koda

Kada Claude na moj zahtev piše neki deo koda, treba da poštuje sledeće:

- `context.Context` kao prvi argument u svim funkcijama koje pristupaju bazi ili mreži.
- Greške se vraćaju eksplicitno; bez panic u HTTP handlerima.
- Greške se omotavaju sa `fmt.Errorf("ntech: imeFunkcije: %w", err)` radi praćenja lanca grešaka.
- Svaki korisnički unos se proverava na nivou handlera pre prosleđivanja poslovnoj logici.
- Osetljivi podaci (lozinke, tokeni, tajne) se ne loguju — loguje se samo ID korisnika i naziv operacije.
- `log/slog` za strukturisano logovanje (JSON u produkciji, tekst u razvoju).
- Konfiguracija preko promenljivih okruženja (lokalno `.env` fajl, nikad u Git).

## HTTP sloj

- Standardna biblioteka `net/http` + `chi` ruter (`github.com/go-chi/chi/v5`).
- Redosled middleware-a (spolja → unutra): recover → real-IP → request-ID → logger → ograničavanje zahteva → provera prijave.
- Sve rute koje zahtevaju prijavu prolaze kroz `RequireAuth` middleware.
- Sve rute koje zahtevaju sesiju potvrđenu 2FA-om prolaze kroz `Require2FA` middleware (kada bude implementiran).
- Statički fajlovi sa odgovarajućim cache zaglavljima; ugrađuju se pomoću `//go:embed` radi isporuke kao jedan binarni fajl.

## Build i pokretanje

```bash
# Razvoj
go run ./cmd/ntech

# Produkcioni build (jedan statički binarni fajl)
CGO_ENABLED=0 go build -o ntech ./cmd/ntech
```

Promenljive okruženja koje program čita pri pokretanju:

| Promenljiva    | Podrazumevano | Opis                                         |
| -------------- | ------------- | -------------------------------------------- |
| `NTECH_ENV`    | `development` | `development` ili `production`               |
| `NTECH_PORT`   | `8080`        | HTTP port za slušanje                        |
| `NTECH_DB`     | `sqlite`      | `sqlite` ili `postgres`                      |
| `NTECH_SQLITE` | `ntech.db`    | Putanja do SQLite fajla                      |
| `NTECH_DSN`    | —             | Postgres connection string                   |
| `NTECH_SECRET` | —             | Ključ za potpisivanje sesija (min. 32 bajta) |

## Testiranje

- Testovi u `_test.go` fajlovima pored koda koji testiraju.
- `testing/quick` ili table-driven testovi za čiste funkcije.
- Za integracione testove SQLite baza u memoriji — bez lažiranja (mock) sloja baze.
- Tokovi prijave treba da imaju integracione testove koji pokrivaju: uspešnu prijavu, pogrešnu lozinku, zaključavanje naloga, aktivaciju 2FA, proveru 2FA, upotrebu rezervnog koda.

## Šta Claude NE SME da radi

- Ne piše ceo program ili veće delove na svoju ruku — radi samo na onome što izričito zatražim.
- Ne proširuje zadatak i ne dodaje delove koje nisam tražio.
- Ne menja delove koda o kojima nismo pričali.
- Ne koristi `database/sql` sa `mattn/go-sqlite3` (zahteva CGO); uvek `modernc.org/sqlite`.
- Ne predlaže čuvanje lozinki ili tajni u običnom tekstu.
- Ne preskače CSRF tokene na POST/PUT/DELETE tačkama.
- Ne uvodi globalno promenljivo stanje van početne konfiguracije.
- Ne koristi `gorilla/sessions` sa cookie skladištem za osetljive podatke — uvek sesije na serveru.
- Ne koristi ORM (bez GORM-a); eksplicitan SQL.
- Ne piše tekst interfejsa na engleskom, hrvatskom, bosanskom ili ijekavskom — sve što korisnik vidi je standardni srpski ekavski.
- Ne piše komentare u kodu na engleskom — komentari su na srpskom.
- Ne prikazuje sirovu Go grešku (npr. `sql: no rows`) direktno u brauzeru — uvek je omotava razumljivom srpskom porukom.

## NAPOMENA

Program mora da bude responsivan da funkcioniše i na svim mobilnim telefonima
