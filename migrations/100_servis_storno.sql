-- Servisni nalog dobija storno (kao Prodaja i Nabavka) — do sada je jedini način
-- da se poništi naplaćen/preuzet nalog bio fizičko brisanje, koje ostavlja mrtve
-- KIR/KPO/fiskalne zapise iza sebe.
ALTER TABLE servisni_nalozi ADD COLUMN stornirano INTEGER NOT NULL DEFAULT 0;
ALTER TABLE servisni_nalozi ADD COLUMN razlog_storniranja TEXT;
