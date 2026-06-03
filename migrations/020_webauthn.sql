CREATE TABLE IF NOT EXISTS webauthn_kredencijali (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    korisnik_id     INTEGER NOT NULL,
    credential_id   TEXT NOT NULL UNIQUE,
    public_key      BLOB NOT NULL,
    sign_count      INTEGER NOT NULL DEFAULT 0,
    datum_kreiranja DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (korisnik_id) REFERENCES korisnici(id) ON DELETE CASCADE
);
