-- Zastavica „uvoz" na zapisima knjige primljenih računa (KPR).
-- Uvoz se unosi RUČNO sa JCI (osnovica i PDV iz carinskog dokumenta drže postojeće
-- kolone osnovica_opsta / pdv_opsta). Zastavica služi da se uvozni PDV u PPPDV obrascu
-- rutira u polja 006/106 (prethodni porez pri uvozu) umesto 008/108 (ostali prethodni).
-- 0 = domaća nabavka (podrazumevano), 1 = uvoz.
ALTER TABLE pdv_kpr ADD COLUMN uvoz INTEGER NOT NULL DEFAULT 0;
