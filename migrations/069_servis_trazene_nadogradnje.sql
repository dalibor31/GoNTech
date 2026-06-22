-- tražene nadogradnje: ono što klijent želi da se dogradi/poboljša,
-- odvojeno od opis_kvara (ono što ne radi)
ALTER TABLE servisni_nalozi ADD COLUMN trazene_nadogradnje TEXT NOT NULL DEFAULT '';
