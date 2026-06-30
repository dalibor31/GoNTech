CREATE TABLE IF NOT EXISTS servisni_log (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    nalog_id   INTEGER NOT NULL REFERENCES servisni_nalozi(id) ON DELETE CASCADE,
    dogadjaj   TEXT    NOT NULL, -- npr. "status:U dijagnostici", "odluka:prihvaceno", "poziv_klijenta"
    napomena   TEXT,
    korisnik_id INTEGER REFERENCES korisnici(id),
    datum      DATETIME NOT NULL DEFAULT (datetime('now','localtime'))
);

CREATE INDEX IF NOT EXISTS idx_servisni_log_nalog ON servisni_log(nalog_id);
