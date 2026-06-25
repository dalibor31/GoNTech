-- Fiskalni računi: veza prodaje sa PFR odgovorom (Teron L-PFR).
-- Svaki prodajni nalog može imati najviše jedan fiskalni račun (UNIQUE prodaja_id).
-- QR kod se čuva kao base64 PNG (dolazi gotov iz Fisk servera).
-- sirovi_odgovor čuva ceo JSON odgovor za audit (sadrži journal, taxItems, itd.).
CREATE TABLE IF NOT EXISTS fiskalni_racuni (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    prodaja_id          INTEGER NOT NULL UNIQUE,
    tip_racuna          TEXT NOT NULL DEFAULT 'Normal',
    tip_transakcije     TEXT NOT NULL DEFAULT 'Sale',
    pfr_broj            TEXT NOT NULL,                -- invoiceNumber iz PFR odgovora
    pfr_vreme           TEXT NOT NULL,                -- sdcDateTime
    brojac              TEXT,                         -- invoiceCounter (npr. "1/1ПП")
    ekstenzija_brojaca  TEXT,                         -- invoiceCounterExtension (npr. "ПП")
    url_verifikacija    TEXT,                         -- verificationUrl
    qr_kod              TEXT,                         -- verificationQRCode (base64 PNG)
    poreske_stavke      TEXT,                         -- JSON niz taxItems[]
    ukupno_za_naplatu   REAL,                         -- totalAmount
    ukupan_porez        REAL,                         -- totalTax
    sirovi_odgovor      TEXT,                         -- ceo JSON odgovor (audit trail)
    potpisao            TEXT,                         -- signedBy
    zatrazio            TEXT,                         -- requestedBy
    poruka              TEXT,                         -- messages
    storniran           INTEGER NOT NULL DEFAULT 0,   -- 1 ako je fiskalni račun storniran
    vreme_kreiranja     TEXT NOT NULL DEFAULT (datetime('now','localtime')),
    FOREIGN KEY (prodaja_id) REFERENCES prodajni_nalozi(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_fiskalni_racuni_prodaja ON fiskalni_racuni(prodaja_id);
CREATE INDEX IF NOT EXISTS idx_fiskalni_racuni_pfr_broj ON fiskalni_racuni(pfr_broj);

-- Podešavanja za fiskalizaciju koja nedostaju u 080
INSERT OR IGNORE INTO podesavanja (kljuc, vrednost) VALUES ('pfr_api_key', '');
INSERT OR IGNORE INTO podesavanja (kljuc, vrednost) VALUES ('pfr_kasir', 'NTech');
