-- nalaz dijagnostike: šta je serviser utvrdio pregledom uređaja (dijagnoza kvara
-- i predlog popravke), odvojeno od „opis kvara" koji je ono što klijent prijavi
ALTER TABLE servisni_nalozi ADD COLUMN nalaz_dijagnostike TEXT NOT NULL DEFAULT '';
