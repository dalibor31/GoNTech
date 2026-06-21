-- Usluge: zaseban entitet (NIJE artikal) — ne prate lager, nemaju nabavnu cenu
-- ni dobavljače. Imaju cenu usluge + PDV stopu. Kategorija je tekstualna oznaka
-- (npr. „Servis", „Instalacija"); slobodan unos uz predlog postojećih vrednosti.
CREATE TABLE IF NOT EXISTS usluge (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    sifra       TEXT,
    naziv       TEXT    NOT NULL,
    kategorija  TEXT    NOT NULL DEFAULT '',
    cena        REAL    NOT NULL DEFAULT 0,
    pdv_stopa   REAL    NOT NULL DEFAULT 20,
    opis        TEXT    NOT NULL DEFAULT '',
    arhiviran   INTEGER NOT NULL DEFAULT 0,
    datum_unosa TEXT    NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_usluge_arhiviran ON usluge(arhiviran);
