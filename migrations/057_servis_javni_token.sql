ALTER TABLE servisni_nalozi ADD COLUMN javni_token TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_servisni_nalozi_javni_token ON servisni_nalozi(javni_token);
