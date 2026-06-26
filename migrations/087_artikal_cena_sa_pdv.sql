ALTER TABLE artikli ADD COLUMN cena_sa_pdv REAL NOT NULL DEFAULT 0;

UPDATE artikli
SET cena_sa_pdv = ROUND(prodajna_cena * (1 + pdv_stopa / 100), 2);
