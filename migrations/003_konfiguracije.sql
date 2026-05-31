CREATE TABLE IF NOT EXISTS konfiguracije (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    artikal_id      INTEGER NOT NULL REFERENCES artikli(id) ON DELETE CASCADE,
    komponenta_id   INTEGER NOT NULL REFERENCES artikli(id) ON DELETE CASCADE,
    kolicina        INTEGER NOT NULL DEFAULT 1
);
