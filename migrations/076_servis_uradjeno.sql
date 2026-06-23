-- uradjeno: serviser upisuje šta je stvarno urađeno na uređaju tokom popravke
-- (zaseban tekst od „nalaz_dijagnostike" koji opisuje utvrđeni kvar i predlog)
ALTER TABLE servisni_nalozi ADD COLUMN uradjeno TEXT NOT NULL DEFAULT '';
