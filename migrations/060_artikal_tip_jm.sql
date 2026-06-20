-- Tip artikla: 'proizvod' (prati lager), 'usluga' i 'trosak' (ne prate lager).
ALTER TABLE artikli ADD COLUMN tip TEXT NOT NULL DEFAULT 'proizvod';
-- Jedinica mere artikla (kom, sat, set, m, l, kg ...).
ALTER TABLE artikli ADD COLUMN jedinica_mere TEXT NOT NULL DEFAULT 'kom';
