-- Sprečava dupliranje kategorije: isti naziv (bez obzira na velika/mala slova)
-- ili isti kôd (prefiks šifre artikla) ne sme postojati kod dve kategorije.
-- Kôd je NULL kad nije postavljen (fallback prefiks ART), pa ga izuzimamo iz
-- provere jedinstvenosti — više kategorija sme biti bez koda.
CREATE UNIQUE INDEX IF NOT EXISTS idx_kategorije_naziv ON kategorije(naziv COLLATE NOCASE);
CREATE UNIQUE INDEX IF NOT EXISTS idx_kategorije_kod ON kategorije(kod) WHERE kod IS NOT NULL;
