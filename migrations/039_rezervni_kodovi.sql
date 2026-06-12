-- Rezervni (jednokratni) kodovi za 2FA — alternativa TOTP-u kada uređaj nije dostupan.
-- Kod se čuva kao bcrypt heš; jednom iskorišćen kod se označava (iskoriscen=1).
CREATE TABLE IF NOT EXISTS rezervni_kodovi (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    korisnik_id      INTEGER  NOT NULL REFERENCES korisnici(id) ON DELETE CASCADE,
    kod_hash         TEXT     NOT NULL,
    iskoriscen       INTEGER  NOT NULL DEFAULT 0,
    datum_kreiranja  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    datum_koriscenja DATETIME
);

CREATE INDEX IF NOT EXISTS idx_rezervni_kodovi_korisnik ON rezervni_kodovi(korisnik_id);
