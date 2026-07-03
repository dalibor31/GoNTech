-- Indeksi na FK kolonama koje se često JOIN-uju/filtriraju (detalji naloga, izveštaji).
-- Bez indeksa ove tabele rade full table scan koji raste sa prometom.
CREATE INDEX IF NOT EXISTS idx_stavke_prodaje_nalog_id ON stavke_prodaje(nalog_id);
CREATE INDEX IF NOT EXISTS idx_stavke_prodaje_artikal_id ON stavke_prodaje(artikal_id);

CREATE INDEX IF NOT EXISTS idx_servisni_delovi_nalog_id ON servisni_delovi(nalog_id);
CREATE INDEX IF NOT EXISTS idx_servisni_delovi_artikal_id ON servisni_delovi(artikal_id);

CREATE INDEX IF NOT EXISTS idx_servisni_potrazivani_delovi_nalog_id ON servisni_potrazivani_delovi(nalog_id);
CREATE INDEX IF NOT EXISTS idx_servisni_potrazivani_delovi_artikal_id ON servisni_potrazivani_delovi(artikal_id);

CREATE INDEX IF NOT EXISTS idx_magacinske_promene_artikal_id ON magacinske_promene(artikal_id);

CREATE INDEX IF NOT EXISTS idx_stavke_nabavke_nabavka_id ON stavke_nabavke(nabavka_id);

CREATE INDEX IF NOT EXISTS idx_pdv_kir_izvor ON pdv_kir(izvor, izvor_id);
CREATE INDEX IF NOT EXISTS idx_pdv_kir_broj_dokumenta ON pdv_kir(broj_dokumenta);

CREATE INDEX IF NOT EXISTS idx_pdv_kpr_izvor ON pdv_kpr(izvor, izvor_id);
