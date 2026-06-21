-- Šifrarnik vrsta troškova (Opcija 1): katalog dodatnih troškova koji se vezuju
-- za radni nalog (npr. Prevoz, Carina, Ambalaža). Ulaze u cenu koštanja naloga,
-- ne na fakturu klijentu. Ovo NIJE knjiga rashoda firme (struja/kirija) — to je
-- poseban modul. Trošak ima samo podrazumevanu cenu; iznos se zadaje pri vezivanju.
CREATE TABLE IF NOT EXISTS troskovi (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    sifra       TEXT,
    naziv       TEXT    NOT NULL,
    cena        REAL    NOT NULL DEFAULT 0,
    opis        TEXT    NOT NULL DEFAULT '',
    arhiviran   INTEGER NOT NULL DEFAULT 0,
    datum_unosa TEXT    NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_troskovi_arhiviran ON troskovi(arhiviran);
