CREATE TABLE IF NOT EXISTS podsetnici (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    naslov              TEXT    NOT NULL,
    napomena            TEXT,
    datum_podsecanja    DATETIME NOT NULL,
    zavrseno            INTEGER NOT NULL DEFAULT 0,
    tip                 TEXT    NOT NULL DEFAULT 'opsti',
    nalog_id            INTEGER,
    tip_naloga          TEXT,
    datum_unosa         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
