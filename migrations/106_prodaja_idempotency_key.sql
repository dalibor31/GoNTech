-- Idempotency ključ za prodajni nalog: frontend generiše UUID po otvaranju forme
-- (jedan ključ po pokušaju unosa) i šalje ga kao skriveno polje. Ako isti POST
-- stigne na server dva puta (dupli klik koji je promakao JS zaštiti, "Nazad" pa
-- ponovni submit, mrežni retry, dva otvorena taba), drugi zahtev se prepoznaje
-- po već postojećem ključu i vraća VEĆ kreirani nalog umesto da napravi drugi.
-- Isti obrazac kao šifra usluge/troška (migracija 105): NULL dozvoljen i ne ulazi
-- u UNIQUE proveru (stari zapisi, ili budući pozivaoci koji ne šalju ključ).
ALTER TABLE prodajni_nalozi ADD COLUMN idempotency_key TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_prodajni_nalozi_idempotency_key
	ON prodajni_nalozi(idempotency_key) WHERE idempotency_key IS NOT NULL;
