CREATE TABLE IF NOT EXISTS servisni_nalozi (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    klijent_id      INTEGER REFERENCES klijenti(id) ON DELETE SET NULL,
    broj_naloga     TEXT    NOT NULL UNIQUE,
    uredjaj         TEXT    NOT NULL,
    serijski_broj   TEXT,
    opis_kvara      TEXT    NOT NULL,
    status          TEXT    NOT NULL DEFAULT 'Primljeno',
    cena_od         REAL,
    cena_do         REAL,
    cena_konacna    REAL,
    avans           REAL,
    napomena        TEXT,
    datum_prijema   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    datum_zavrsetka DATETIME
);
