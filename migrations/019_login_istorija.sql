CREATE TABLE IF NOT EXISTS login_istorija (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    korisnik_id INTEGER,
    ip          TEXT NOT NULL DEFAULT '',
    user_agent  TEXT NOT NULL DEFAULT '',
    uspeh       INTEGER NOT NULL DEFAULT 0,
    razlog      TEXT NOT NULL DEFAULT '',
    vreme       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (korisnik_id) REFERENCES korisnici(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_login_istorija_korisnik ON login_istorija(korisnik_id, vreme);
