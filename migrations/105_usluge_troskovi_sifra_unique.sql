-- Šifra usluge/troška treba da bude jedinstvena, isti obrazac kao artikal
-- šifra/barkod (migracija 055_artikal_sifra_barkod.sql). Prazna šifra ('') se
-- ne čuva (handler šalje NULL kad je polje prazno), pa je izuzimamo da više
-- usluga/troškova bez šifre ne bi sudarilo proveru.
CREATE UNIQUE INDEX IF NOT EXISTS idx_usluge_sifra ON usluge(sifra) WHERE sifra IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_troskovi_sifra ON troskovi(sifra) WHERE sifra IS NOT NULL;
