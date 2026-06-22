-- napomena klijentu: poruka koju serviser ostavlja klijentu (vidljiva na nalogu),
-- odvojeno od interne napomene
ALTER TABLE servisni_nalozi ADD COLUMN napomena_klijentu TEXT NOT NULL DEFAULT '';
