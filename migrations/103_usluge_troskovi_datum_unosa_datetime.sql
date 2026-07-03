-- usluge.datum_unosa i troskovi.datum_unosa su bile deklarisane kao TEXT, za
-- razliku od svih ostalih tabela (DATETIME) — modernc.org/sqlite driver zato
-- vraća goli string umesto time.Time pri Scan-u. Fizički format podataka se ne
-- menja (SQLite nema stvarni numerički afinitet za DATETIME nad ovakvim
-- stringom), samo deklarisani tip kolone — pa je rekreacija tabele bezbedna
-- bez transformacije vrednosti.

CREATE TABLE usluge_novo (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    sifra         TEXT,
    naziv         TEXT    NOT NULL,
    kategorija    TEXT    NOT NULL DEFAULT '',
    jedinica_mere TEXT    NOT NULL DEFAULT 'usluga',
    cena          REAL    NOT NULL DEFAULT 0,
    pdv_stopa     REAL    NOT NULL DEFAULT 20,
    opis          TEXT    NOT NULL DEFAULT '',
    arhiviran     INTEGER NOT NULL DEFAULT 0,
    datum_unosa   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO usluge_novo (id, sifra, naziv, kategorija, jedinica_mere, cena, pdv_stopa, opis, arhiviran, datum_unosa)
    SELECT id, sifra, naziv, kategorija, jedinica_mere, cena, pdv_stopa, opis, arhiviran, datum_unosa FROM usluge;
DROP TABLE usluge;
ALTER TABLE usluge_novo RENAME TO usluge;
CREATE INDEX IF NOT EXISTS idx_usluge_arhiviran ON usluge(arhiviran);

CREATE TABLE troskovi_novo (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    sifra       TEXT,
    naziv       TEXT    NOT NULL,
    cena        REAL    NOT NULL DEFAULT 0,
    opis        TEXT    NOT NULL DEFAULT '',
    arhiviran   INTEGER NOT NULL DEFAULT 0,
    datum_unosa DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO troskovi_novo (id, sifra, naziv, cena, opis, arhiviran, datum_unosa)
    SELECT id, sifra, naziv, cena, opis, arhiviran, datum_unosa FROM troskovi;
DROP TABLE troskovi;
ALTER TABLE troskovi_novo RENAME TO troskovi;
CREATE INDEX IF NOT EXISTS idx_troskovi_arhiviran ON troskovi(arhiviran);
