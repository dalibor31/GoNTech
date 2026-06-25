-- Popravka: fiskalni_racuni mora da podrži i servisne naloge (pored prodajnih).
-- Originalna migracija 081 imala je FOREIGN KEY na prodajni_nalozi(id),
-- ali servisni nalozi ne idu u tu tabelu. Zamenjujemo FK sa nullable kolonom.
-- Rekreiramo tabelu: kopiramo podatke, brišemo staru, kreiramo novu bez FK.

-- 1. Nova tabela bez FK — prodaja_id je nullable
CREATE TABLE IF NOT EXISTS fiskalni_racuni_new (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    prodaja_id          INTEGER,                     -- NULL za servisne naloge
    servis_id           INTEGER,                     -- NULL za prodajne naloge
    tip_racuna          TEXT NOT NULL DEFAULT 'Normal',
    tip_transakcije     TEXT NOT NULL DEFAULT 'Sale',
    pfr_broj            TEXT NOT NULL,
    pfr_vreme           TEXT NOT NULL,
    brojac              TEXT,
    ekstenzija_brojaca  TEXT,
    url_verifikacija    TEXT,
    qr_kod              TEXT,
    poreske_stavke      TEXT,
    ukupno_za_naplatu   REAL,
    ukupan_porez        REAL,
    sirovi_odgovor      TEXT,
    potpisao            TEXT,
    zatrazio            TEXT,
    poruka              TEXT,
    storniran           INTEGER NOT NULL DEFAULT 0,
    vreme_kreiranja     TEXT NOT NULL DEFAULT (datetime('now','localtime'))
);

-- 2. Kopiraj postojeće redove (ako ih ima) iz stare tabele
INSERT INTO fiskalni_racuni_new
    (id, prodaja_id, tip_racuna, tip_transakcije, pfr_broj, pfr_vreme,
     brojac, ekstenzija_brojaca, url_verifikacija, qr_kod,
     poreske_stavke, ukupno_za_naplatu, ukupan_porez,
     sirovi_odgovor, potpisao, zatrazio, poruka, storniran, vreme_kreiranja)
SELECT id, prodaja_id, tip_racuna, tip_transakcije, pfr_broj, pfr_vreme,
       brojac, ekstenzija_brojaca, url_verifikacija, qr_kod,
       poreske_stavke, ukupno_za_naplatu, ukupan_porez,
       sirovi_odgovor, potpisao, zatrazio, poruka, storniran, vreme_kreiranja
FROM fiskalni_racuni;

-- 3. Zameni tabele
DROP TABLE fiskalni_racuni;
ALTER TABLE fiskalni_racuni_new RENAME TO fiskalni_racuni;

-- 4. Indeksi
CREATE INDEX IF NOT EXISTS idx_fiskalni_racuni_prodaja ON fiskalni_racuni(prodaja_id);
CREATE INDEX IF NOT EXISTS idx_fiskalni_racuni_servis ON fiskalni_racuni(servis_id);
CREATE INDEX IF NOT EXISTS idx_fiskalni_racuni_pfr_broj ON fiskalni_racuni(pfr_broj);
