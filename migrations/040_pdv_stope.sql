-- Šifarnik PDV stopa (Faza 1 knjigovodstvenog modula).
-- Stope se čuvaju kao podatak, ne hardkod — nova ili izmenjena stopa bez diranja koda.
-- Početne vrednosti po važećem zakonu (jun 2026): opšta 20%, posebna 10%, oslobođeno 0%.
CREATE TABLE IF NOT EXISTS pdv_stope (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    naziv       TEXT    NOT NULL,           -- npr. "Opšta stopa"
    stopa       REAL    NOT NULL,           -- procenat, npr. 20.0
    oznaka      TEXT    NOT NULL,           -- "opsta" | "posebna" | "oslobodjeno"
    aktivna     INTEGER NOT NULL DEFAULT 1, -- 1 = u upotrebi, 0 = arhivirana
    redosled    INTEGER NOT NULL DEFAULT 0, -- redosled prikaza u listama
    datum_unosa DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Seed početnih stopa samo ako je tabela prazna (idempotentno, ne gazi ručne izmene).
INSERT INTO pdv_stope (naziv, stopa, oznaka, redosled)
SELECT naziv, stopa, oznaka, redosled FROM (
    SELECT 'Opšta stopa'     AS naziv, 20.0 AS stopa, 'opsta'       AS oznaka, 1 AS redosled
    UNION ALL SELECT 'Posebna stopa',   10.0, 'posebna',     2
    UNION ALL SELECT 'Oslobođeno (0%)',  0.0, 'oslobodjeno', 3
)
WHERE NOT EXISTS (SELECT 1 FROM pdv_stope);
