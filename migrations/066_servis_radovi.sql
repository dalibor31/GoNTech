-- Radovi (usluge) na servisnom nalogu: stavke rada izabrane iz cenovnika usluga.
-- Naziv i cena se snapshot-uju (kopiraju) na nalog jer se cena može menjati po
-- nalogu i usluga u cenovniku može kasnije biti izmenjena. Radovi NE diraju lager.
-- Zbir radova = cena rada na nalogu.
CREATE TABLE IF NOT EXISTS servis_radovi (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    nalog_id    INTEGER NOT NULL REFERENCES servisni_nalozi(id) ON DELETE CASCADE,
    usluga_id   INTEGER,
    naziv       TEXT    NOT NULL,
    kolicina    REAL    NOT NULL DEFAULT 1,
    cena_komada REAL    NOT NULL DEFAULT 0,
    datum       TEXT    NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_servis_radovi_nalog ON servis_radovi(nalog_id);
