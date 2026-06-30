-- Prodaja: popust na stavci (rabat u procentima)
-- Neophodno za B2B fakture; PDV se obračunava na iznos posle popusta

ALTER TABLE stavke_prodaje ADD COLUMN popust_procenat REAL NOT NULL DEFAULT 0;
