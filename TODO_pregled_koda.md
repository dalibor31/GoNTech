# TODO — opšti pregled koda (NTech)

Pregled urađen bez izmena koda (samo istraga). Fokus: dual-render forme, transakciona
ispravnost repository sloja, handler validacija/CSRF/dozvole, neslaganja model↔baza↔šablon,
JS logika (duple registracije, double-submit zaštita), i ostalo sumnjivo.

Legenda ozbiljnosti: 🔴 visok · 🟡 srednji · 🟢 nizak

> **Napomena o mestu ovog fajla (2026-07-31):** ostaje u root-u repoa, ne premešta se u `docs/`.
> `docs/` folder je na ovoj mašini u celosti isključen iz git-a preko `.git/info/exclude` (lokalna,
> lična konfiguracija — nije `.gitignore`, pa se ne deli sa timom/remote-om) — premeštanje ovog
> fajla tamo bi ga tiho izbacilo iz verzionisane istorije. Isto važi za `BUG.md` (postojeći,
> opsežniji numerisani spisak nalaza projekta) i sve `AGENTS.md`/`CLAUDE.md` fajlove — svi su
> lokalno-isključeni na ovoj mašini. Nalazi #45–#47 iz ove serije (dupli POST na prodaji, IDOR na
> podsetnicima, idempotency za servis) su zato, radi konzistentnosti, dodati i u `BUG.md` (ista
> numeracija/format kao ostali nalazi tog fajla) — ali ta izmena, kao ni ostale izmene AGENTS.md
> fajlova (osvežen broj migracija i LOC, dokumentovan `idempotency_key` i `data-full-reload`
> obrazac), NIJE deo git istorije na ovoj mašini zbog gorenavedenog isključenja. Ovaj fajl
> (`TODO_pregled_koda.md`) ostaje jedini **verzionisani** zapis ove serije ispravki.

---

## 1. Dual-submit / duple registracije event listenera

- [x] 🔴 **`prodaja_forma.html` — dva nezavisna 'submit' listenera na istoj formi, mogući DUPLI POST po jednom kliku. ISPRAVLJENO.**
  Dodat `data-full-reload` atribut na `<form>` (`web/templates/stranice/prodaja_forma.html:35`) — generički AJAX submit-interceptor u `base.html:401` sad tu formu preskače (rani `return`), pa jedini submit-put ostaje Alpine-ov `@submit.prevent="posaljiProdaju($event)"` (native `e.target.submit()`). Potvrđeno statičkom analizom (browser extension nije bio dostupan u ovoj sesiji za live Network-tab proveru): pre izmene, `f.hasAttribute('data-full-reload')` je bilo `false` za ovu formu pa bi generički handler nastavio na `fetch()` POST — sad je `true` i handler odmah izlazi. Proverено `curl`-om da se atribut ispravno renderuje u serviranom HTML-u (`/prodaja/nova`).
  `web/templates/stranice/prodaja_forma.html:35` — `<form method="POST" action="/prodaja/nova" @submit.prevent="posaljiProdaju($event)">` nema `data-full-reload` atribut.
  `web/templates/teme/podrazumevana/base.html:396-410` — generički AJAX submit-interceptor se kači na **SVAKU** `form[method="POST"]` u dokumentu (osim multipart ili `data-full-reload`), uključujući ovu formu, i pri submit-u odmah šalje sopstveni `fetch()` POST.
  `web/static/js/ntech.js:400-410` (`posaljiProdaju`) — Alpine-ov `@submit.prevent` handler ODVOJENO zakazuje `e.target.submit()` (pravi native POST, pun page-reload) preko `$nextTick`.
  Pošto nijedan od handlera ne zove `stopImmediatePropagation()`, oba se izvršavaju na isti klik → **dva POST zahteva** ka `/prodaja/nova` (jedan preko `fetch`, jedan preko native submit-a). Ovo je najverovatniji PRAVI uzrok originalno prijavljenog bug-a "prodaja se upiše dva puta" — raniji nalaz (nedostajući `:disabled` na `pdv_stopa[]`, već ispravljen) je stvaran, ali verovatno NIJE bio glavni uzrok dupliranja REDOVA u bazi.
  Napomena: upravo dodati `idempotency_key` (migracija 106, `ProdajaRepo.Kreiraj`) verovatno already prikriva VIDLJIV simptom (drugi zahtev se sad prepoznaje i vraća isti nalog) — ali dupli HTTP zahtev i dalje postoji (nepotrebno opterećenje, mogući flash pogrešnog tosta/greške ako prvi fetch stigne sa nepočišćenim praznim stavkama pre nego što Alpine stigne da ih filtrira).
  Postojeća konvencija za ovakav slučaj već postoji u repo-u: `data-full-reload` (v. `nabavka_detalji.html:197`, `pdv_kir.html:29`, `kpo.html:24` itd.) — forma bi trebalo da dobije taj atribut da se isključi iz generičkog handlera, ili se custom `@submit.prevent` mehanizam treba ukloniti u korist generičkog.
  **Preporuka**: potvrditi u browseru (Network tab) da li se zaista šalju 2 zahteva pri klik na "Naplati", pa dodati `data-full-reload` na `<form>` (ili ukloniti dupli mehanizam).

- [x] 🟢 Ostale forme (`nabavka_forma.html`, `servis_forma.html`, itd.) ne koriste `@submit` Alpine direktivu (potvrđeno: `grep -rln "@submit" web/templates/` vraća samo `prodaja_forma.html`) — generički AJAX handler u `base.html` je jedini submit-put za njih, nema konflikta. Bez akcije.

---

## 2. Dual-render forme (desktop tabela + mobilna kartica) — `:disabled="isMobile"` obrazac

- [x] ✅ **`prodaja_forma.html:131,223` — `pdv_stopa[]` bez `:disabled` — VEĆ ISPRAVLJENO** u prethodnom zadatku (dodato `:disabled="isMobile"` / `:disabled="!isMobile"` analogno susednim poljima).

- [x] 🟢 `nabavka_forma.html` (jedina druga stranica sa `isMobile` dual-render obrascem, `stavke` blok) — svi `[]` inputi (`artikal_id[]`, `kolicina[]`, `cena_po_komadu[]`, `marza[]`, `prodajna[]`) imaju simetričan `:disabled="isMobile"`/`:disabled="!isMobile"` par (redovi 135-161 desktop, 208-233 mobilni). **Nema iste greške.**
  Zavisni troškovi (`trosak_naziv[]`, `trosak_iznos[]`, redovi 274-278) NISU dual-render (samo jedan `x-for` blok, bez posebne mobilne kopije) — nema rizika duplog slanja.

- [x] 🟢 Nijedna druga `stranice/*.html` stranica ne koristi `isMobile` dual-render Alpine obrazac (servis, klijenti, artikli/magacin liste koriste responsive CSS/HTMX pristup, ne duplo renderovanje `x-for` sa `:disabled` prekidačem) — obrazac bug-a je specifičan za prodaju/nabavku, oba pokrivena.

---

## 3. Repository sloj — transakciona ispravnost i race conditions

- [x] Svih **20** `BeginTx(...)` poziva u `internal/db/sqlite/*.go` ima `defer tx.Rollback()` odmah posle provere greške (provereno u: `artikal.go`, `rezervni_kodovi.go`, `servisni_potrazivani_delovi.go`, `nabavka.go`, `prodaja.go`, `nivelacija.go`, `servisni_radovi.go`, `servisni_delovi.go`, `servis.go`) — **nema propusta**, bez akcije.

- [x] 🟡 **`ServisRepo.Kreiraj` nema idempotency zaštitu — ISPRAVLJENO.**
  Primenjen isti obrazac kao za prodaju (migracija 106): nova migracija `migrations/107_servis_idempotency_key.sql` (kolona `idempotency_key` + parcijalni `UNIQUE` indeks na `servisni_nalozi`), `model.ServisniNalog.IdempotencyKey` polje, `ServisRepo.Kreiraj` (`internal/db/sqlite/servis.go`) proverava postojeći ključ pre insert-a i vraća postojeći ID ako je nalog već kreiran istim ključem, `parseFormuNaloga` (`internal/handler/servis.go`) čita `idempotency_key` iz forme, `servis_forma.html` dobija skriveno polje + JS UUID generator (samo za novi nalog, ne za izmenu). Dodat `internal/db/sqlite/servis_idempotency_test.go` (2 testa: sa ključem vraća isti ID, bez ključa prave se dva odvojena naloga kao ranije) — oba prolaze.

- [x] 🟢 `NabavkaRepo.Kreiraj` (`internal/db/sqlite/nabavka.go:178`) nema interni sekvencijalni broj (koristi `broj_racuna` dobavljača, korisnički unet) — nema race-a oko generisanja broja, ali isti opšti rizik "dva odvojena POST-a = dve nabavke" postoji arhitekturno kao i svuda gde nema idempotency ključa. Niži prioritet jer nema poznatu žalbu/simptom.
  **PREOSTALO — nije automatski rađeno.** Isti obrazac (idempotency_key kolona + parcijalni UNIQUE indeks) bi se mogao primeniti i ovde radi pune konzistentnosti sa prodajom/servisom — javljeno koordinatoru na odluku, nije prioritet jer nema poznat simptom.

- [x] 🟢 Parametrizacija SQL upita — nasumičan pregled `fmt.Sprintf`/string-concat obrazaca u `internal/db/sqlite/*.go` (magacin.go:112, artikal.go:167,191, klijent.go, trosak.go, usluga.go) — svi slučajevi concat-uju samo **hardkodovane** identifikatore kolona/tabela (const liste, fiksni literali iz poziva) ili grade `?` placeholder nizove; korisnički unos ide isključivo kroz `args`. **Nema SQL injection rizika** u pregledanim mestima.

---

## 4. Handler sloj — validacija, CSRF, autorizacija

- [x] 🔴 **IDOR / broken access control na podsetnicima (ličnim podsetnicima). ISPRAVLJENO.**
  Dodata `korisnikSmeDaMenjaPodsetnik(k, p)` provera (`internal/handler/podsetnici.go`) — dozvoljava izmenu/završavanje/brisanje ako je korisnik admin/superadmin (`middleware.JeAdmin`) ili je `p.KorisnikID` tačno njegov ID; u suprotnom `403 Forbidden`. Primenjeno u sve tri funkcije: `SacuvajIzmenePodsetnika` (sad prvo učitava postojeći podsetnik pre izmene), `OznaciPodsetnik` i `ObrisiPodsetnik` (sad prvo učitava podsetnik pre brisanja, ranije je brisao direktno po ID-u bez čitanja). Dodat `internal/handler/podsetnici_test.go` sa 6 test-slučajeva (radnik/tuđi radnik/admin × svoj/tuđi/nedodeljen podsetnik) — svi prolaze.
  `internal/handler/podsetnici.go`:
  - `SacuvajIzmenePodsetnika` (red 166-195) — učitava `id` iz URL-a, poziva `PodsetnikRepo.Izmeni` BEZ provere da `podsetnik.KorisnikID` pripada ulogovanom korisniku (ili je izmenilac admin/superadmin).
  - `OznaciPodsetnik` (red 198-218) — isto, menja status završenosti bilo kog podsetnika po ID-u bez provere vlasništva.
  - `ObrisiPodsetnik` (red 221-234) — isto, briše bilo koji podsetnik po ID-u bez provere vlasništva.
  Lista (`filter.KorisnikID = &k.ID`, red 51) i kreiranje ISPRAVNO vezuju podsetnik za korisnika (osim kad admin/superadmin eksplicitno dodeli drugom korisniku, red 260-269), ali IZMENA/ZAVRŠI/BRISANJE nemaju tu proveru — bilo koji ulogovan korisnik (uključujući najniže privilegovanu ulogu) može menjati/brisati/označavati kao završene tuđe (pa i admin/superadmin) podsetnike prostim pogađanjem/iteracijom numeričkog ID-a u POST ruti.
  **Preporuka**: u sve tri funkcije dohvatiti postojeći podsetnik, proveriti `postojeci.KorisnikID == &k.ID` (ili `k.Uloga` je admin/superadmin), vratiti 403/redirect sa greškom ako ne odgovara — isti obrazac kao provera vlasništva koja bi trebalo da postoji za bilo koji "moj" resurs.

- [x] 🟢 CSRF pokrivenost rutera (`cmd/ntech/main.go`) — sve mutating rute (`POST/PUT/DELETE`) su unutar jednog od dva `r.Group` bloka koji uključuju `ntechmw.CsrfMiddleware` (redovi 255-263 javne, 285-515 zaštićene), OSIM `/status/{token}/prihvati|odbij|odluka-odabrano` (redovi 270-281) — ovo je **namerno i dokumentovano** (komentar red 265-268): autentikacija je jednokratni tajni token u URL-u, capability model, ne sesijski kolačić, pa klasičan CSRF model napada ne važi. Logika je zdrava, bez akcije, samo zabeleženo radi potpunosti pregleda.

- [x] 🟢 Raw Go greške korisniku — pregled `http.Error(w, err.Error(), ...)` / sličnih obrazaca u `internal/handler/*.go`: **nije pronađen nijedan slučaj** curenja sirove Go greške korisniku; svuda se koriste fiksne srpske poruke uz `slog.Error` za internu dijagnostiku. Bez akcije.

- [x] 🟢 Sistematski `_ = ...`/`x, _ = ...` obrasci u `internal/handler/*.go` (npr. `podsetnici.go:91,154,287` — `korisnici, _ = h.KorisniciRepo.Lista(...)` za popunu dropdown-a; `prijava.go` — best-effort audit logovanje neuspešnih pokušaja prijave) — namerno "best-effort" ponašanje (log/dropdown ne sme blokirati glavni tok), prihvatljivo. Jedini blagi rizik: ako `KorisniciRepo.Lista` padne, dropdown za dodelu podsetnika drugom korisniku će tiho biti prazan bez indikacije greške korisniku — kozmetički, nizak prioritet.
  **PREOSTALO — nije automatski rađeno.** Kozmetička izmena (npr. `slog.Warn` pri grešci) bi se mogla dodati — javljeno koordinatoru na odluku, vrlo nizak prioritet.

---

## 5. Neslaganja model ↔ baza ↔ šablon

- [x] 🟢 Spot-provera `model.ProdajniNalog` ↔ `SELECT` u `ProdajaRepo.DohvatiID` (`internal/db/sqlite/prodaja.go:121-147`) — polja se poklapaju 1:1 (uklj. i novo `IdempotencyKey`, namerno izostavljeno iz SELECT-a jer se ne prikazuje nigde — ne predstavlja bug). Nije pronađeno neslaganje na ovom mestu.
  Dublja/iscrpna provera SVIH modul-tabela-šablon trojki nije urađena u ovom prolazu zbog obima repoa (~40+ tabela) — vredi ponoviti ciljano po modulu ako se posumnja na konkretno polje.

---

## 6. Ostalo

- [x] 🟢 Nema zaostalih `TODO`/`FIXME`/`XXX`/`HACK` komentara u `internal/`, `web/`, `cmd/` (pretraga bez pogotka) — kod je već "čist" po tom kriterijumu.
- [x] 🟢 Data-refresh re-izvršavanje `<script>` blokova (`base.html:437-441`) — ugrađeni inline skriptovi unutar `data-refresh` regiona (npr. `servis_detalji.html` naplata-dijalog skripta oko reda 100-234) su ispravno vezani za NOVI DOM čvor pri svakom `replaceWith`, pa ne dolazi do akumulacije listenera na istom, trajnom elementu — obrazac je ispravan, bez akcije.

---

## Sažetak

Status posle ispravke: sva 🔴 visok i 🟡 srednji rizik nalaza su ISPRAVLJENA (3/3). Svi 🟢 nizak
rizik nalazi su ili potvrde "bez akcije" (kod je već ispravan) ili su prijavljeni koordinatoru
kao preostali, po dogovoru bez automatske izmene.

| Kategorija | Nalaza | 🔴 Visok | 🟡 Srednji | 🟢 Nizak |
|---|---|---|---|---|
| 1. Dual-submit / listeneri | 2 | 1 ✅ | 0 | 1 |
| 2. Dual-render forme | 3 | 0 | 0 | 3 (1 ✅ ispravljen ranije) |
| 3. Repository / transakcije | 4 | 0 | 1 ✅ | 3 (1 preostalo) |
| 4. Handler / CSRF / dozvole | 4 | 1 ✅ | 0 | 3 (1 preostalo) |
| 5. Model↔baza↔šablon | 1 | 0 | 0 | 1 |
| 6. Ostalo | 2 | 0 | 0 | 2 |
| **Ukupno** | **16** | **2/2 ✅** | **1/1 ✅** | **13** (2 preostala, niska prioriteta) |

**Ispravljeni nalazi (visok/srednji rizik):**
1. ✅ **Dupli POST na `/prodaja/nova`** — dodat `data-full-reload` na formu, isključuje je iz generičkog AJAX interceptora u `base.html`; jedini submit-put ostaje Alpine `@submit.prevent`. Commit: `db1b551`.
2. ✅ **IDOR na podsetnicima** — dodata `korisnikSmeDaMenjaPodsetnik` provera vlasništva u `SacuvajIzmenePodsetnika`, `OznaciPodsetnik`, `ObrisiPodsetnik`; 403 ako korisnik nije vlasnik niti admin/superadmin. Commit: `b9e7043`.
3. ✅ **`ServisRepo.Kreiraj` bez idempotency zaštite** — dodat isti obrazac kao za prodaju (migracija 107, `idempotency_key` kolona + parcijalni UNIQUE indeks, provera u `Kreiraj`, skriveno polje u `servis_forma.html`). Commit: `3998119`.

**Preostalo (nizak rizik, nije automatski rađeno — čeka odluku):**
- `NabavkaRepo.Kreiraj` nema isti idempotency obrazac (nema poznat simptom, niži prioritet).
- Silent-fail dropdown u `podsetnici.go` (`korisnici, _ = ...`) — kozmetička izmena, vrlo nizak prioritet.
