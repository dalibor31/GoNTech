-- predviđen datum popravke: ručni override roka za pojedinačni nalog.
-- Ako je popunjen, koristi se on; ako je NULL, prikaz računa izvedeni default
-- (datum_prijema + predvidjen_rok_dana iz podešavanja).
ALTER TABLE servisni_nalozi ADD COLUMN predvidjen_datum DATE;
