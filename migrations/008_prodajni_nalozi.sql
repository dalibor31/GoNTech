CREATE TABLE IF NOT EXISTS prodajni_nalozi (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    klijent_id      INTEGER REFERENCES klijenti(id) ON DELETE SET NULL,
    broj_naloga     TEXT    NOT NULL UNIQUE,
    napomena        TEXT,
    ukupno          REAL    NOT NULL DEFAULT 0,
    datum           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
