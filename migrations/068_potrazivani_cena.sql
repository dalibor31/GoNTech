-- cena po komadu za potraživani deo, da bi pri pokrivanju (kad roba stigne)
-- deo mogao da se prebaci u ugrađene delove sa ispravnom cenom
ALTER TABLE servisni_potrazivani_delovi ADD COLUMN cena_komada REAL NOT NULL DEFAULT 0;
