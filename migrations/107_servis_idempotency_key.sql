-- Idempotency ključ za servisni nalog — isti obrazac kao za prodaju
-- (migracija 106_prodaja_idempotency_key.sql): frontend generiše UUID po otvaranju
-- forme i šalje ga kao skriveno polje. Ako isti POST stigne na server dva puta
-- (dupli klik, "Nazad" pa ponovni submit, mrežni retry, dva otvorena taba),
-- drugi zahtev se prepoznaje po već postojećem ključu i vraća VEĆ kreirani nalog
-- umesto da napravi drugi. NULL dozvoljen i ne ulazi u UNIQUE proveru (stari
-- zapisi, ili budući pozivaoci koji ne šalju ključ).
ALTER TABLE servisni_nalozi ADD COLUMN idempotency_key TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_servisni_nalozi_idempotency_key
	ON servisni_nalozi(idempotency_key) WHERE idempotency_key IS NOT NULL;
