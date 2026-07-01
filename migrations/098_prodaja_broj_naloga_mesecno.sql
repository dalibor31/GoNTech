-- Prodaja: broj naloga menja format iz PR-GGGG-NNNN (puna godina, brojač po godini)
-- u PR-GGMM-NNNN (dvocifrena godina + mesec, brojač po mesecu) — usklađeno sa
-- servisnim nalozima koji već koriste mesečni brojač.
--
-- Idempotentno: pogađa samo redove čiji četvorocifreni blok NIJE validan mesec na
-- poslednje dve cifre (stari format: puna godina npr. "2026" → poslednje dve cifre
-- "26" nisu 01-12; novi format: "2601" → "01" jeste validan mesec — pa migracija na
-- ponovnom pokretanju ništa ne dira).

WITH stari AS (
    SELECT id, datum FROM prodajni_nalozi
    WHERE CAST(substr(broj_naloga, 6, 2) AS INTEGER) NOT BETWEEN 1 AND 12
),
raspored AS (
    SELECT id,
        'PR-' || substr(datum, 3, 2) || substr(datum, 6, 2) || '-' ||
        printf('%04d', ROW_NUMBER() OVER (
            PARTITION BY substr(datum, 1, 7)
            ORDER BY datum, id
        )) AS novi_broj
    FROM stari
)
UPDATE prodajni_nalozi
SET broj_naloga = (SELECT novi_broj FROM raspored WHERE raspored.id = prodajni_nalozi.id)
WHERE id IN (SELECT id FROM raspored);

-- uskladi denormalizovane kopije broja dokumenta u KIR/KPO sa novim brojem naloga
UPDATE pdv_kir
SET broj_dokumenta = (SELECT broj_naloga FROM prodajni_nalozi WHERE id = pdv_kir.izvor_id)
WHERE izvor = 'prodaja' AND izvor_id IN (SELECT id FROM prodajni_nalozi);

UPDATE kpo_unosi
SET broj_dokumenta = (SELECT broj_naloga FROM prodajni_nalozi WHERE id = kpo_unosi.izvor_id)
WHERE izvor = 'prodaja' AND izvor_id IN (SELECT id FROM prodajni_nalozi);
