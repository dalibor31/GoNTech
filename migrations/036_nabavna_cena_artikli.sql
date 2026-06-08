-- Dodaje nabavnu cenu u artikle ako već ne postoji.
-- Potrebno jer je migration 002 mogla biti preskočena na bazama koje su
-- imale tabelu artikli pre uvođenja te migracije.
ALTER TABLE artikli ADD COLUMN nabavna_cena REAL NOT NULL DEFAULT 0;
