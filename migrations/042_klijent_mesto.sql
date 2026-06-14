-- Dodaje mesto/grad u klijente. Koristi se pri izboru klijenta u KIR formi
-- (popunjava mesto kupca), a korisno je i samo po sebi za adresu klijenta.
ALTER TABLE klijenti ADD COLUMN mesto TEXT;
