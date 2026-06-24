-- komentar klijenta uz odluku o predlogu (prihvatanje/odbijanje)
ALTER TABLE servisni_nalozi ADD COLUMN komentar_klijenta TEXT DEFAULT '';
