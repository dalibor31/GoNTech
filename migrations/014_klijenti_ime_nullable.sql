-- uklanja NOT NULL sa ime i prezime jer klijent može biti samo firma
CREATE TABLE IF NOT EXISTS klijenti_novi (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    ime             TEXT,
    prezime         TEXT,
    naziv_firme     TEXT,
    pib             TEXT,
    telefon         TEXT,
    email           TEXT,
    napomena        TEXT,
    datum_unosa     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO klijenti_novi SELECT id, ime, prezime, naziv_firme, pib, telefon, email, napomena, datum_unosa FROM klijenti;

DROP TABLE klijenti;

ALTER TABLE klijenti_novi RENAME TO klijenti;
