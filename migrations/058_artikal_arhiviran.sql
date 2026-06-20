-- Arhiviranje artikala: artikal koji je bio u prometu ne može se obrisati (FK RESTRICT),
-- pa se umesto brisanja označava kao arhiviran. Arhiviran artikal se ne nudi za nov promet,
-- ali ostaje vidljiv u istoriji (prodaja, nabavka, servis) i na svojoj kartici.
ALTER TABLE artikli ADD COLUMN arhiviran INTEGER NOT NULL DEFAULT 0;
