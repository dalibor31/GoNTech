CREATE TABLE IF NOT EXISTS stavke_prodaje (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    nalog_id        INTEGER NOT NULL REFERENCES prodajni_nalozi(id) ON DELETE CASCADE,
    artikal_id      INTEGER NOT NULL REFERENCES artikli(id) ON DELETE RESTRICT,
    kolicina        INTEGER NOT NULL DEFAULT 1,
    cena_po_komadu  REAL    NOT NULL,
    ukupno          REAL    NOT NULL
);
