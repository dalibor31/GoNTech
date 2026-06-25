-- Retroaktivni upis u KIR za postojeće servisne naloge koji su već preuzeti.
-- Ranije se KIR nije automatski popunjavao za servis; ova migracija dodaje zapise
-- za sve naloge sa statusom "Preuzeto" koji imaju naplaćen iznos > 0 i nisu odbijene popravke.
-- PDV stopa: 20% (opšta) za sve servisne radove i ugrađene delove.

INSERT INTO pdv_kir (datum_prometa, datum_knjizenja, broj_dokumenta,
                      kupac_naziv, kupac_pib, kupac_mesto,
                      osnovica_opsta, pdv_opsta, osnovica_posebna, pdv_posebna,
                      ukupno, izvor, izvor_id)
SELECT
    COALESCE(sn.datum_zavrsetka, sn.datum_prijema)       AS datum_prometa,
    COALESCE(sn.datum_zavrsetka, sn.datum_prijema)       AS datum_knjizenja,
    sn.broj_naloga                                        AS broj_dokumenta,
    COALESCE(kp.naziv, '')                                AS kupac_naziv,
    COALESCE(k.pib, '')                                   AS kupac_pib,
    COALESCE(k.mesto, '')                                 AS kupac_mesto,
    ROUND(sn.naplaceno / 1.2, 2)                         AS osnovica_opsta,
    ROUND(sn.naplaceno - (sn.naplaceno / 1.2), 2)        AS pdv_opsta,
    0                                                     AS osnovica_posebna,
    0                                                     AS pdv_posebna,
    sn.naplaceno                                          AS ukupno,
    'servis'                                              AS izvor,
    sn.id                                                 AS izvor_id
FROM servisni_nalozi sn
LEFT JOIN klijent_prikaz kp ON kp.id = sn.klijent_id
LEFT JOIN klijenti k ON k.id = sn.klijent_id
WHERE sn.status = 'Preuzeto'
  AND sn.naplaceno > 0
  AND sn.popravka_odbijena = 0
  AND NOT EXISTS (
      SELECT 1 FROM pdv_kir pk
      WHERE pk.izvor = 'servis' AND pk.izvor_id = sn.id
  );
