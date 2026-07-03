-- Zamrzava nabavnu cenu na stavci prodaje i ugrađenom servisnom delu u trenutku
-- unosa — potrebno za obračun prave zarade (cena − nabavna cena) u Izveštajima
-- bez retroaktivnog preračunavanja ako se artikli.nabavna_cena kasnije promeni.
ALTER TABLE stavke_prodaje ADD COLUMN nabavna_cena REAL NOT NULL DEFAULT 0;
ALTER TABLE servisni_delovi ADD COLUMN nabavna_cena REAL NOT NULL DEFAULT 0;

UPDATE stavke_prodaje
SET nabavna_cena = COALESCE((SELECT a.nabavna_cena FROM artikli a WHERE a.id = stavke_prodaje.artikal_id), 0);

UPDATE servisni_delovi
SET nabavna_cena = COALESCE((SELECT a.nabavna_cena FROM artikli a WHERE a.id = servisni_delovi.artikal_id), 0);
