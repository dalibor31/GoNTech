-- Proširuje CHECK constraint na magacinske_promene da uključi manjak_popis i visak_popis
-- SQLite ne podržava ALTER TABLE za CHECK, pa rekreiramo tabelu
PRAGMA foreign_keys=OFF;

CREATE TABLE magacinske_promene_nova (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    artikal_id       INTEGER NOT NULL REFERENCES artikli(id) ON DELETE RESTRICT,
    tip_promene      TEXT    NOT NULL CHECK (tip_promene IN (
                         'ulaz_nabavka', 'izlaz_prodaja', 'izlaz_servis',
                         'povracaj', 'korekcija', 'manjak_popis', 'visak_popis'
                     )),
    referentni_id    INTEGER NOT NULL,
    promena_kolicine INTEGER NOT NULL,
    stanje_pre       INTEGER NOT NULL,
    stanje_posle     INTEGER NOT NULL,
    korisnik_id      INTEGER REFERENCES korisnici(id) ON DELETE SET NULL,
    napomena         TEXT,
    datum            DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO magacinske_promene_nova SELECT * FROM magacinske_promene;
DROP TABLE magacinske_promene;
ALTER TABLE magacinske_promene_nova RENAME TO magacinske_promene;

PRAGMA foreign_keys=ON;
