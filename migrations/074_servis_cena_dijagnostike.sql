-- cena dijagnostike: taksa koja se naplaćuje kada klijent ne prihvati popravku
-- posle dijagnostike; ulazi u ukupno za naplatu samo kada je veća od 0
ALTER TABLE servisni_nalozi ADD COLUMN cena_dijagnostike REAL NOT NULL DEFAULT 0;
