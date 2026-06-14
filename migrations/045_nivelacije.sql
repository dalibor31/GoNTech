-- Nivelacije (Faza A kalkulacije/nivelacije): trag svake promene prodajne cene artikla.
-- Razlika (nova − stara) se izvodi u programu, ne čuva se. Izvor razlikuje način promene:
-- 'rucno' (posebna akcija sa razlogom), 'izmena' (auto pri izmeni artikla), 'kalkulacija' (kasnije).
CREATE TABLE IF NOT EXISTS nivelacije (
    id            INTEGER  PRIMARY KEY AUTOINCREMENT,
    artikal_id    INTEGER  NOT NULL REFERENCES artikli(id) ON DELETE CASCADE,
    stara_cena    REAL     NOT NULL,
    nova_cena     REAL     NOT NULL,
    razlog        TEXT,
    izvor         TEXT     NOT NULL DEFAULT 'rucno',
    korisnik_id   INTEGER  REFERENCES korisnici(id) ON DELETE SET NULL,
    datum         DATE     NOT NULL,                       -- poslovni datum promene
    datum_unosa   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_nivelacije_artikal ON nivelacije(artikal_id);
CREATE INDEX IF NOT EXISTS idx_nivelacije_datum ON nivelacije(datum);
