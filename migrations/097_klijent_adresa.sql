-- Dodaje adresu (ulica i broj) u klijente. Zakon o PDV (čl. 42, sadržaj računa)
-- zahteva adresu kupca na fakturi za pravna lica — mesto (grad) samo nije dovoljno.
ALTER TABLE klijenti ADD COLUMN adresa TEXT;
