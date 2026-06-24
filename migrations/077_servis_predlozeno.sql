-- predlozeno: razdvaja stavke unete pri prijemu (zahtev klijenta, predlozeno=0)
-- od onih koje serviser predlaže posle dijagnostike (predlozeno=1).
-- Predložene stavke klijent vidi obojeno na status stranici i odobrava ih;
-- prihvatanjem prelaze u ugrađene (predlozeno → 0).
ALTER TABLE servisni_delovi ADD COLUMN predlozeno INTEGER NOT NULL DEFAULT 0;
ALTER TABLE servis_radovi ADD COLUMN predlozeno INTEGER NOT NULL DEFAULT 0;
ALTER TABLE servisni_potrazivani_delovi ADD COLUMN predlozeno INTEGER NOT NULL DEFAULT 0;
