-- Zavisni troškovi nabavke (carina, prevoz, špedicija…) i metod njihove raspodele.
-- Troškovi se raspodeljuju na stavke i ulaze u kalkulativnu nabavnu cenu, iz koje
-- se računa prodajna. Slobodne stavke (naziv + iznos), više redova po nabavci.

-- metod raspodele po nabavci: 'vrednost' (po nabavnoj vrednosti stavke) ili 'kolicina'.
-- NULL/prazno = nema zavisnih troškova (kalkulacija kao u Fazi B).
ALTER TABLE nabavke ADD COLUMN metod_raspodele TEXT;

-- pojedinačne stavke zavisnih troškova; brišu se zajedno sa nabavkom (CASCADE)
CREATE TABLE IF NOT EXISTS nabavka_troskovi (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    nabavka_id INTEGER NOT NULL REFERENCES nabavke(id) ON DELETE CASCADE,
    naziv      TEXT NOT NULL,
    iznos      REAL NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_nabavka_troskovi_nabavka ON nabavka_troskovi(nabavka_id);
