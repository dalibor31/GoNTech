-- Servis: broj naloga menja redosled iz SN-MMGG-NNN (mesec-godina) u SN-GGMM-NNN
-- (godina-mesec) — usklađeno sa prodajnim nalozima (PR-GGMM-NNNN, migracija 098).
--
-- Idempotentno: broj se za svaki nalog PONOVO izračunava iz datum_prijema (isti
-- hronološki raspored po mesecu daje isti rezultat svaki put), pa se UPDATE
-- primenjuje samo na redove čija se trenutna vrednost razlikuje od izračunate.

WITH raspored AS (
    SELECT id,
        'SN-' || substr(datum_prijema, 3, 2) || substr(datum_prijema, 6, 2) || '-' ||
        printf('%03d', ROW_NUMBER() OVER (
            PARTITION BY substr(datum_prijema, 1, 7)
            ORDER BY datum_prijema, id
        )) AS novi_broj
    FROM servisni_nalozi
)
UPDATE servisni_nalozi
SET broj_naloga = (SELECT novi_broj FROM raspored WHERE raspored.id = servisni_nalozi.id)
WHERE id IN (
    SELECT raspored.id FROM raspored
    JOIN servisni_nalozi sn ON sn.id = raspored.id
    WHERE raspored.novi_broj != sn.broj_naloga
);

-- uskladi denormalizovane kopije broja dokumenta u KIR/KPO sa novim brojem naloga
UPDATE pdv_kir
SET broj_dokumenta = (SELECT broj_naloga FROM servisni_nalozi WHERE id = pdv_kir.izvor_id)
WHERE izvor = 'servis' AND izvor_id IN (SELECT id FROM servisni_nalozi);

UPDATE kpo_unosi
SET broj_dokumenta = (SELECT broj_naloga FROM servisni_nalozi WHERE id = kpo_unosi.izvor_id)
WHERE izvor = 'servis' AND izvor_id IN (SELECT id FROM servisni_nalozi);
