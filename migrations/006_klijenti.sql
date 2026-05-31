CREATE TABLE IF NOT EXISTS klijenti (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    ime             TEXT    NOT NULL,
    prezime         TEXT    NOT NULL,
    naziv_firme     TEXT,
    pib             TEXT,
    telefon         TEXT,
    email           TEXT,
    napomena        TEXT,
    datum_unosa     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
