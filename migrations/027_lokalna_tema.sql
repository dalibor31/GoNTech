-- globalna tema u podešavanjima (podrazumevano tamna)
INSERT INTO podesavanja (kljuc, vrednost)
VALUES ('globalna_tema', 'tamna')
ON CONFLICT(kljuc) DO NOTHING;

-- lokalna tema korisnika
ALTER TABLE korisnici ADD COLUMN lokalna_tema TEXT;
ALTER TABLE korisnici ADD COLUMN koristi_lokalnu_temu INTEGER NOT NULL DEFAULT 0;
