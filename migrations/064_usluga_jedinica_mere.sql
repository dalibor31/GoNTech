-- Jedinica mere usluge: 'usluga' (fiksna), 'sat' (naplata po satu) ili 'kom'.
ALTER TABLE usluge ADD COLUMN jedinica_mere TEXT NOT NULL DEFAULT 'usluga';
