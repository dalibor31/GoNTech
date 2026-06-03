CREATE TABLE IF NOT EXISTS pokusaji_prijave (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    ip              TEXT    NOT NULL,
    korisnicko_ime  TEXT    NOT NULL DEFAULT '',
    uspeh           INTEGER NOT NULL DEFAULT 0,
    vreme           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_pokusaji_ip_vreme ON pokusaji_prijave(ip, vreme);
