-- Podešavanja za automatski backup baze.
-- backup_interval_sati: na koliko sati se pravi automatska rezervna kopija (uz onu pri pokretanju).
-- backup_broj_kopija: koliko poslednjih kopija se čuva (starije se brišu — rotacija).
INSERT OR IGNORE INTO podesavanja (kljuc, vrednost) VALUES ('backup_interval_sati', '24');
INSERT OR IGNORE INTO podesavanja (kljuc, vrednost) VALUES ('backup_broj_kopija', '7');
