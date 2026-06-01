-- uklanjamo nabavnu cenu iz artikala
-- SQLite ne podržava DROP COLUMN direktno pre verzije 3.35
-- koristimo standardni pristup: nova tabela, kopiranje, brisanje stare
CREATE TABLE artikli_novi (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    kategorija_id   INTEGER REFERENCES kategorije(id) ON DELETE SET NULL,
    naziv           TEXT    NOT NULL,
    opis            TEXT,
    kolicina        INTEGER NOT NULL DEFAULT 0,
    kolicina_min    INTEGER NOT NULL DEFAULT 0,
    lokacija        TEXT,
    prodajna_cena   REAL    NOT NULL DEFAULT 0,
    napomena        TEXT,
    datum_unosa     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO artikli_novi
    (id, kategorija_id, naziv, opis, kolicina, kolicina_min,
     lokacija, prodajna_cena, napomena, datum_unosa)
SELECT
    id, kategorija_id, naziv, opis, kolicina, kolicina_min,
    lokacija, prodajna_cena, napomena, datum_unosa
FROM artikli;

DROP TABLE artikli;
ALTER TABLE artikli_novi RENAME TO artikli;

-- tabela nabavki
CREATE TABLE IF NOT EXISTS nabavke (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    dobavljac_id    INTEGER REFERENCES dobavljaci(id) ON DELETE SET NULL,
    broj_nabavke    TEXT    NOT NULL UNIQUE,
    napomena        TEXT,
    ukupno          REAL    NOT NULL DEFAULT 0,
    datum           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- stavke nabavke
CREATE TABLE IF NOT EXISTS stavke_nabavke (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    nabavka_id      INTEGER NOT NULL REFERENCES nabavke(id) ON DELETE CASCADE,
    artikal_id      INTEGER NOT NULL REFERENCES artikli(id) ON DELETE RESTRICT,
    kolicina        INTEGER NOT NULL DEFAULT 1,
    cena_po_komadu  REAL    NOT NULL,
    ukupno          REAL    NOT NULL
);
