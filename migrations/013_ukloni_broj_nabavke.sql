-- uklanjamo kolonu broj_nabavke iz tabele nabavke
-- SQLite ne dozvoljava DROP COLUMN na koloni sa UNIQUE ograničenjem,
-- pa koristimo standardni pristup: nova tabela, kopiranje, brisanje stare, preimenovanje

CREATE TABLE nabavke_novi (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    dobavljac_id INTEGER REFERENCES dobavljaci(id) ON DELETE SET NULL,
    napomena     TEXT,
    ukupno       REAL    NOT NULL DEFAULT 0,
    datum        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO nabavke_novi (id, dobavljac_id, napomena, ukupno, datum)
SELECT id, dobavljac_id, napomena, ukupno, datum
FROM nabavke;

DROP TABLE nabavke;
ALTER TABLE nabavke_novi RENAME TO nabavke;
