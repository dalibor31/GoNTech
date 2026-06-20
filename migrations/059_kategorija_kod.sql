-- Kôd kategorije služi kao prefiks šifre artikla (npr. "Komponente" -> KOMP -> KOMP-0001).
ALTER TABLE kategorije ADD COLUMN kod TEXT;
