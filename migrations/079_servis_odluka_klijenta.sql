-- odluka klijenta o predloženim delovima/uslugama (prihvatanje/odbijanje)
ALTER TABLE servisni_nalozi ADD COLUMN odluka_klijenta TEXT DEFAULT '';
ALTER TABLE servisni_nalozi ADD COLUMN datum_odluke DATETIME;
