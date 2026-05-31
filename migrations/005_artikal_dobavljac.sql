CREATE TABLE IF NOT EXISTS artikal_dobavljac (
    artikal_id      INTEGER NOT NULL REFERENCES artikli(id) ON DELETE CASCADE,
    dobavljac_id    INTEGER NOT NULL REFERENCES dobavljaci(id) ON DELETE CASCADE,
    napomena        TEXT,
    PRIMARY KEY (artikal_id, dobavljac_id)
);
