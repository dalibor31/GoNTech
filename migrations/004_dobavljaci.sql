CREATE TABLE IF NOT EXISTS dobavljaci (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    naziv           TEXT    NOT NULL,
    kontakt_osoba   TEXT,
    telefon         TEXT,
    email           TEXT,
    website         TEXT,
    napomena        TEXT,
    datum_unosa     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
