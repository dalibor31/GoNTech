-- Popuni cena_konacna za postojeće „Preuzeto" naloge:
-- dijagnostika + radovi + delovi
UPDATE servisni_nalozi
SET cena_konacna = COALESCE(cena_dijagnostike, 0)
    + COALESCE((SELECT SUM(cena_komada * kolicina) FROM servis_radovi WHERE nalog_id = servisni_nalozi.id), 0)
    + COALESCE((SELECT SUM(cena_komada * kolicina) FROM servisni_delovi WHERE nalog_id = servisni_nalozi.id), 0)
WHERE status = 'Preuzeto'
  AND cena_konacna IS NULL;
