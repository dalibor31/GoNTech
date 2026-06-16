-- lična avatar slika korisnika (prazno = koristi inicijale)
ALTER TABLE korisnici ADD COLUMN avatar_putanja TEXT NOT NULL DEFAULT '';
