-- View za prikazni naziv klijenta — objedinjuje logiku koja se ranije ponavljala
-- u upitima prodaje, servisa, dashboarda i izveštaja.
-- Za pravno lice vraća naziv firme; za fizičko "ime prezime"; inače prazan string.
CREATE VIEW IF NOT EXISTS klijent_prikaz AS
SELECT id,
       COALESCE(NULLIF(naziv_firme, ''),
                TRIM(COALESCE(ime, '') || ' ' || COALESCE(prezime, '')), '') AS naziv
FROM klijenti;
