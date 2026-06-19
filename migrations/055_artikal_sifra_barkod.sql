ALTER TABLE artikli ADD COLUMN sifra TEXT;
ALTER TABLE artikli ADD COLUMN barkod TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_artikli_sifra ON artikli(sifra) WHERE sifra IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_artikli_barkod ON artikli(barkod) WHERE barkod IS NOT NULL;
