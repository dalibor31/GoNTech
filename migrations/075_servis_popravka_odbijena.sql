-- popravka_odbijena: označava nalog kod kog je klijent posle dijagnostike odbio popravku
-- (nalog ide u „Završeno", naplaćuje se samo dijagnostika, radovi i delovi se ne gledaju)
ALTER TABLE servisni_nalozi ADD COLUMN popravka_odbijena INTEGER NOT NULL DEFAULT 0;
