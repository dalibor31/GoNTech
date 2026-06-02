CREATE TABLE IF NOT EXISTS sesije (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    korisnik_id       INTEGER NOT NULL REFERENCES korisnici(id) ON DELETE CASCADE,
    token             TEXT    NOT NULL UNIQUE,
    totp_potvrdjeno   INTEGER NOT NULL DEFAULT 1,
    datum_isteka      DATETIME NOT NULL,
    datum_kreiranja   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
