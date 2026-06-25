-- Servis: način plaćanja i naplaćeni iznos pri preuzimanju uređaja.
-- Koristi se za evidenciju i fiskalizaciju pri prelasku u status "Preuzeto".
ALTER TABLE servisni_nalozi ADD COLUMN nacin_placanja TEXT DEFAULT '';
ALTER TABLE servisni_nalozi ADD COLUMN naplaceno REAL DEFAULT 0;
