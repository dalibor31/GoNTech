-- garancija_dana: trajanje garancije u danima (računato od datuma završetka radova).
-- Unosi se dok je uređaj u radu; pravi datum (garancija_do) se popunjava kad nalog
-- pređe u „Završeno" (datum_zavrsetka + garancija_dana). NULL ili 0 = bez garancije.
ALTER TABLE servisni_nalozi ADD COLUMN garancija_dana INTEGER;
