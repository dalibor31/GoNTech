-- ============================================
-- Faza 1: POS proširenje — PDV, magacinska revizija, servisni delovi
-- ============================================

-- 1. Artikli — dodajemo PDV stopu (nabavna_cena već postoji)
ALTER TABLE artikli ADD COLUMN pdv_stopa REAL NOT NULL DEFAULT 20.0;

-- 2. Stavke prodaje — PDV po stavci (za tačan obračun i buduće fiskalne račune)
ALTER TABLE stavke_prodaje ADD COLUMN pdv_stopa    REAL NOT NULL DEFAULT 20.0;
ALTER TABLE stavke_prodaje ADD COLUMN pdv_iznos    REAL NOT NULL DEFAULT 0;
ALTER TABLE stavke_prodaje ADD COLUMN cena_bez_pdv REAL NOT NULL DEFAULT 0;

-- 3. Prodajni nalozi — način plaćanja i storno
ALTER TABLE prodajni_nalozi ADD COLUMN nacin_placanja     TEXT NOT NULL DEFAULT 'gotovina'
    CHECK (nacin_placanja IN ('gotovina', 'kartica', 'prenos'));
ALTER TABLE prodajni_nalozi ADD COLUMN stornirano          INTEGER NOT NULL DEFAULT 0;
ALTER TABLE prodajni_nalozi ADD COLUMN razlog_storniranja  TEXT;

-- 4. Servisni nalozi — tehničar i garancija
ALTER TABLE servisni_nalozi ADD COLUMN tehnicar_id  INTEGER REFERENCES korisnici(id) ON DELETE SET NULL;
ALTER TABLE servisni_nalozi ADD COLUMN garancija_do DATE;

-- 5. Klijenti — JMBG i tip (fizičko/pravno lice)
ALTER TABLE klijenti ADD COLUMN jmbg TEXT;
ALTER TABLE klijenti ADD COLUMN tip  TEXT NOT NULL DEFAULT 'fizicko'
    CHECK (tip IN ('fizicko', 'pravno'));

-- 6. Magacinska revizija — trag svake promene stanja artikla
CREATE TABLE IF NOT EXISTS magacinske_promene (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    artikal_id       INTEGER NOT NULL REFERENCES artikli(id) ON DELETE RESTRICT,
    tip_promene      TEXT    NOT NULL CHECK (tip_promene IN (
                         'ulaz_nabavka', 'izlaz_prodaja', 'izlaz_servis', 'povracaj', 'korekcija'
                     )),
    referentni_id    INTEGER NOT NULL,
    promena_kolicine INTEGER NOT NULL,
    stanje_pre       INTEGER NOT NULL,
    stanje_posle     INTEGER NOT NULL,
    korisnik_id      INTEGER REFERENCES korisnici(id) ON DELETE SET NULL,
    napomena         TEXT,
    datum            DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 7. Servisni delovi — artikli ugrađeni u servisni nalog
CREATE TABLE IF NOT EXISTS servisni_delovi (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    nalog_id    INTEGER NOT NULL REFERENCES servisni_nalozi(id) ON DELETE CASCADE,
    artikal_id  INTEGER NOT NULL REFERENCES artikli(id) ON DELETE RESTRICT,
    kolicina    INTEGER NOT NULL DEFAULT 1 CHECK (kolicina > 0),
    cena_komada REAL    NOT NULL,
    datum       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
