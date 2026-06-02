CREATE TABLE IF NOT EXISTS korisnici (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    korisnicko_ime  TEXT    NOT NULL UNIQUE,
    lozinka_hash    TEXT    NOT NULL,
    uloga           TEXT    NOT NULL DEFAULT 'radnik',
    aktivan         INTEGER NOT NULL DEFAULT 1,
    totp_tajna      TEXT,
    datum_kreiranja DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
