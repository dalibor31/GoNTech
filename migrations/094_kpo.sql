-- KPO: knjiga o ostvarenom prometu (za paušalce i preduzetnike prostog knjigovodstva).
-- Svaki red = jedan prihod (prodajni nalog, naplaćeni servisni nalog ili ručni unos).
-- izvor: 'rucno' | 'prodaja' | 'servis'
-- izvor_id: id izvornog naloga (NULL za ručni unos)
CREATE TABLE IF NOT EXISTS kpo_unosi (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    datum_prometa   DATE    NOT NULL,       -- datum prometa / naplate
    redni_broj      INTEGER,                -- redni broj u knjizi (opciono, može biti auto)
    broj_dokumenta  TEXT    NOT NULL,       -- broj računa / naloga
    opis            TEXT,                   -- opis prometa
    prihod          REAL    NOT NULL DEFAULT 0, -- ukupan prihod (sa PDV-om ako je u sistemu PDV-a)
    nacin_placanja  TEXT,                   -- Gotovina, Kartica, Virman...
    napomena        TEXT,
    izvor           TEXT    NOT NULL DEFAULT 'rucno',
    izvor_id        INTEGER,                -- id prodajnog ili servisnog naloga
    datum_unosa     DATETIME NOT NULL DEFAULT (datetime('now','localtime'))
);

CREATE INDEX IF NOT EXISTS idx_kpo_datum ON kpo_unosi(datum_prometa);
CREATE INDEX IF NOT EXISTS idx_kpo_izvor ON kpo_unosi(izvor, izvor_id);
