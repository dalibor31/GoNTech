-- Veza KIR/KPR zapisa sa izvorom (Faza 2b — automatsko punjenje).
-- izvor: 'rucno' (ručni unos), 'prodaja', 'nabavka'; izvor_id = id naloga/nabavke.
-- Postojeći zapisi dobijaju 'rucno' (DEFAULT). Omogućava brisanje vezanog zapisa
-- pri stornu/brisanju izvora i sprečava duplikate.
ALTER TABLE pdv_kir ADD COLUMN izvor TEXT NOT NULL DEFAULT 'rucno';
ALTER TABLE pdv_kir ADD COLUMN izvor_id INTEGER;
ALTER TABLE pdv_kpr ADD COLUMN izvor TEXT NOT NULL DEFAULT 'rucno';
ALTER TABLE pdv_kpr ADD COLUMN izvor_id INTEGER;
