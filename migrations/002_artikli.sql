CREATE TABLE IF NOT EXISTS artikli (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    kategorija_id   INTEGER REFERENCES kategorije(id) ON DELETE SET NULL,
    naziv           TEXT    NOT NULL,
    opis            TEXT,
    kolicina        INTEGER NOT NULL DEFAULT 0,
    kolicina_min    INTEGER NOT NULL DEFAULT 0,
    lokacija        TEXT,
    nabavna_cena    REAL    NOT NULL DEFAULT 0,
    prodajna_cena   REAL    NOT NULL DEFAULT 0,
    napomena        TEXT,
    datum_unosa     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
