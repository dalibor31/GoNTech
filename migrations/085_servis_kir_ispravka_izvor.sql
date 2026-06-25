-- Ispravka: postojeći KIR zapisi za servisne naloge koji su greškom upisani sa
-- izvor = 'prodaja' — prepravljamo ih u izvor = 'servis'.
-- Zatim brišemo potencijalne duplikate koje je migracija 084 eventualno napravila
-- (kad je dodala novi red sa izvorom 'servis' pored postojećeg sa 'prodaja').

-- 1. Ispravi izvor sa 'prodaja' na 'servis' za SN- naloge
UPDATE pdv_kir SET izvor = 'servis'
WHERE izvor = 'prodaja'
  AND broj_dokumenta LIKE 'SN-%'
  AND izvor_id IN (SELECT id FROM servisni_nalozi WHERE status = 'Preuzeto');

-- 2. Obriši duplikate: ako za isti izvor_id postoji više redova, zadrži samo jedan
DELETE FROM pdv_kir
WHERE rowid NOT IN (
    SELECT MIN(rowid) FROM pdv_kir
    WHERE izvor = 'servis'
    GROUP BY izvor_id
)
AND izvor = 'servis';
