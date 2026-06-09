# Project.md - POS + Servis Sistem za Računarsku Radnju (Go)

## Opis projekta

Sistem za vođenje servisa računara i maloprodaje delova, alata, kablova, komponenti i gotovih računara/laptopova. Podržava:
- Magacinsko poslovanje
- Maloprodaju (sa fiskalnom kasom)
- Servisne naloge
- Nabavke od dobavljača
- Upravljanje klijentima (fizička i pravna lica)
- Izveštaje za poresku upravu

---

## Arhitektura baze podataka (PostgreSQL)

```sql
-- =============================================
-- 1. Preduzeće (tvoja firma)
-- =============================================
CREATE TABLE preduzeca (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    naziv           VARCHAR(255) NOT NULL,
    adresa          TEXT NOT NULL,
    pib             VARCHAR(9) UNIQUE NOT NULL,
    maticni_broj    VARCHAR(8) UNIQUE NOT NULL,
    pdv_broj        VARCHAR(15),
    sifra_delatnosti VARCHAR(5) NOT NULL,
    racun_u_banci   VARCHAR(18),
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- =============================================
-- 2. Korisnici sistema
-- =============================================
CREATE TABLE korisnici (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    preduzece_id    UUID NOT NULL REFERENCES preduzeca(id) ON DELETE CASCADE,
    korisnicko_ime  VARCHAR(100) UNIQUE NOT NULL,
    hash_lozinke    VARCHAR(255) NOT NULL,
    uloga           VARCHAR(50) NOT NULL CHECK (uloga IN ('admin', 'menadzer', 'kasir', 'tehnicar')),
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- =============================================
-- 3. Dobavljači
-- =============================================
CREATE TABLE dobavljaci (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    naziv           VARCHAR(255) NOT NULL,
    adresa          TEXT,
    pib             VARCHAR(9) UNIQUE,
    je_u_pdv_sistemu BOOLEAN DEFAULT TRUE,
    kontakt_osoba   VARCHAR(100),
    telefon         VARCHAR(20),
    email           VARCHAR(255),
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- =============================================
-- 4. Klijenti (fizička i pravna lica)
-- =============================================
CREATE TABLE klijenti (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tip             VARCHAR(20) NOT NULL CHECK (tip IN ('fizicko_lico', 'pravno_lico')),
    ime             VARCHAR(100),
    prezime         VARCHAR(100),
    jmbg            VARCHAR(13) UNIQUE,
    naziv_firme     VARCHAR(255),
    pib             VARCHAR(9) UNIQUE,
    adresa          TEXT,
    telefon         VARCHAR(20),
    email           VARCHAR(255),
    lojalnost_bodovi INT DEFAULT 0,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- =============================================
-- 5. Proizvodi (delovi, alati, kablovi, komponente, računari)
-- =============================================
CREATE TABLE proizvodi (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sifra               VARCHAR(100) UNIQUE NOT NULL,
    naziv               VARCHAR(255) NOT NULL,
    opis                TEXT,
    kategorija          VARCHAR(100) NOT NULL,
    proizvodjac         VARCHAR(100),
    nabavna_cena_bez_pdv DECIMAL(15,2) NOT NULL,
    prodajna_cena_sa_pdv DECIMAL(15,2) NOT NULL,
    pdv_stopa           DECIMAL(5,2) DEFAULT 20.00,
    kolicina_na_stanju  INT NOT NULL DEFAULT 0,
    minimalna_kolicina  INT DEFAULT 5,
    lokacija            VARCHAR(50),
    garancija_meseci    INT DEFAULT 12,
    aktivan             BOOLEAN DEFAULT TRUE,
    created_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- =============================================
-- 6. Servisni nalozi
-- =============================================
CREATE TABLE servisni_nalozi (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    broj_naloga         VARCHAR(50) UNIQUE NOT NULL,
    klijent_id          UUID NOT NULL REFERENCES klijenti(id),
    tip_uredjaja        VARCHAR(50) NOT NULL,
    marka_uredjaja      VARCHAR(100),
    model_uredjaja      VARCHAR(100),
    serijski_broj       VARCHAR(100),
    prijavljeni_kvar    TEXT NOT NULL,
    dijagnoza           TEXT,
    status              VARCHAR(50) NOT NULL CHECK (status IN ('na_cekanju', 'dijagnostika', 'ceka_delove', 'popravka', 'testiranje', 'gotov', 'zavrsen', 'odbijen')),
    cena_rada_bez_pdv   DECIMAL(15,2) DEFAULT 0,
    cena_rada_sa_pdv    DECIMAL(15,2) DEFAULT 0,
    predvidjeni_zavrsetak DATE,
    zavrsen_datuma      TIMESTAMP,
    primio              UUID REFERENCES korisnici(id),
    tehnicar_id         UUID REFERENCES korisnici(id),
    napomene_klijenta   TEXT,
    interne_napomene    TEXT,
    created_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- =============================================
-- 7. Ugrađeni delovi u servisu
-- =============================================
CREATE TABLE servisni_delovi (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    servisni_nalog_id   UUID NOT NULL REFERENCES servisni_nalozi(id) ON DELETE CASCADE,
    proizvod_id         UUID NOT NULL REFERENCES proizvodi(id),
    kolicina            INT NOT NULL CHECK (kolicina > 0),
    cena_komada_bez_pdv DECIMAL(15,2) NOT NULL,
    created_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- =============================================
-- 8. Prodajne transakcije (fiskalni računi)
-- =============================================
CREATE TABLE prodaje (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    broj_prodaje        VARCHAR(50) UNIQUE NOT NULL,
    klijent_id          UUID REFERENCES klijenti(id),
    prodavac_id         UUID NOT NULL REFERENCES korisnici(id),
    tip_prodaje         VARCHAR(20) NOT NULL CHECK (tip_prodaje IN ('maloprodaja_fizicko', 'maloprodaja_pravno', 'servis')),
    fiskalni_broj_racuna VARCHAR(50),
    fiskalni_qr_kod     TEXT,
    nacin_placanja      VARCHAR(20) NOT NULL CHECK (nacin_placanja IN ('gotovina', 'kartica', 'virman')),
    ukupno_bez_pdv      DECIMAL(15,2) NOT NULL,
    ukupno_pdv          DECIMAL(15,2) NOT NULL,
    ukupno_sa_pdv       DECIMAL(15,2) NOT NULL,
    datum_prodaje       TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    stornirano          BOOLEAN DEFAULT FALSE,
    razlog_storniranja  TEXT
);

-- =============================================
-- 9. Stavke prodaje
-- =============================================
CREATE TABLE stavke_prodaje (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    prodaja_id          UUID NOT NULL REFERENCES prodaje(id) ON DELETE CASCADE,
    proizvod_id         UUID NOT NULL REFERENCES proizvodi(id),
    kolicina            INT NOT NULL CHECK (kolicina > 0),
    cena_komada_bez_pdv DECIMAL(15,2) NOT NULL,
    cena_komada_sa_pdv  DECIMAL(15,2) NOT NULL,
    iznos_pdv           DECIMAL(15,2) NOT NULL,
    ukupno_bez_pdv      DECIMAL(15,2) NOT NULL,
    ukupno_sa_pdv       DECIMAL(15,2) NOT NULL
);

-- =============================================
-- 10. Nabavke od dobavljača
-- =============================================
CREATE TABLE nabavke (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    broj_nabavke        VARCHAR(50) UNIQUE NOT NULL,
    dobavljac_id        UUID NOT NULL REFERENCES dobavljaci(id),
    broj_fakture        VARCHAR(100),
    datum_fakture       DATE,
    tip_nabavke         VARCHAR(20) NOT NULL CHECK (tip_nabavke IN ('domaca', 'uvozna')),
    carinska_deklaracija VARCHAR(100),
    ukupno_bez_pdv      DECIMAL(15,2) NOT NULL,
    ukupno_pdv          DECIMAL(15,2) NOT NULL,
    ukupno_sa_pdv       DECIMAL(15,2) NOT NULL,
    created_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- =============================================
-- 11. Stavke nabavke
-- =============================================
CREATE TABLE stavke_nabavke (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    nabavka_id          UUID NOT NULL REFERENCES nabavke(id) ON DELETE CASCADE,
    proizvod_id         UUID NOT NULL REFERENCES proizvodi(id),
    kolicina            INT NOT NULL CHECK (kolicina > 0),
    cena_komada_bez_pdv DECIMAL(15,2) NOT NULL,
    pdv_stopa           DECIMAL(5,2) NOT NULL,
    ukupno_bez_pdv      DECIMAL(15,2) NOT NULL,
    ukupno_pdv          DECIMAL(15,2) NOT NULL
);

-- =============================================
-- 12. Magacinske promene (revizija)
-- =============================================
CREATE TABLE magacinske_promene (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    proizvod_id         UUID NOT NULL REFERENCES proizvodi(id),
    tip_promene         VARCHAR(20) NOT NULL CHECK (tip_promene IN ('ulaz_nabavka', 'izlaz_prodaja', 'izlaz_servis', 'povracaj_ulaz', 'povracaj_izlaz', 'korekcija')),
    referentni_id       UUID NOT NULL,
    promena_kolicine    INT NOT NULL,
    stanje_pre          INT NOT NULL,
    stanje_posle        INT NOT NULL,
    cena_komada_bez_pdv DECIMAL(15,2) NOT NULL,
    created_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    kreirao             UUID REFERENCES korisnici(id)
);
