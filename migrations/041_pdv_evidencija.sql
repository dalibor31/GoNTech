-- PDV evidencija (Faza 2a knjigovodstvenog modula): knjiga izdatih računa (KIR)
-- i knjiga primljenih računa (KPR). Iznosi se vode po VRSTI stope (opšta/posebna),
-- ne po procentu — promena stope zakonom ne razbija kolone.
-- Napomena: ovo je osnovna knjiga; pun POPDV obrazac (uvoz, avansi, poljoprivreda)
-- dolazi u kasnijoj iteraciji.

-- Knjiga izdatih računa (izlazni PDV).
CREATE TABLE IF NOT EXISTS pdv_kir (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    datum_prometa       DATE    NOT NULL,           -- datum prometa / izdavanja računa
    datum_knjizenja     DATE    NOT NULL,           -- datum knjiženja u KIR
    broj_dokumenta      TEXT    NOT NULL,           -- broj računa ili drugog dokumenta
    kupac_naziv         TEXT    NOT NULL,
    kupac_pib           TEXT,                       -- PIB ili JMBG kupca
    kupac_mesto         TEXT,
    osnovica_opsta      REAL    NOT NULL DEFAULT 0, -- osnovica oporeziva opštom stopom
    pdv_opsta           REAL    NOT NULL DEFAULT 0, -- obračunati PDV po opštoj stopi
    osnovica_posebna    REAL    NOT NULL DEFAULT 0, -- osnovica oporeziva posebnom stopom
    pdv_posebna         REAL    NOT NULL DEFAULT 0, -- obračunati PDV po posebnoj stopi
    osloboden_sa_pravom REAL    NOT NULL DEFAULT 0, -- oslobođen promet sa pravom na odbitak
    osloboden_bez_prava REAL    NOT NULL DEFAULT 0, -- oslobođen promet bez prava na odbitak
    ukupno              REAL    NOT NULL DEFAULT 0, -- ukupna naknada sa PDV
    napomena            TEXT,
    datum_unosa         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Knjiga primljenih računa (ulazni PDV).
CREATE TABLE IF NOT EXISTS pdv_kpr (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    datum_prometa       DATE    NOT NULL,           -- datum prometa / prijema računa
    datum_knjizenja     DATE    NOT NULL,           -- datum knjiženja u KPR
    datum_placanja      DATE,                       -- datum plaćanja (može biti prazan)
    broj_dokumenta      TEXT    NOT NULL,
    dobavljac_naziv     TEXT    NOT NULL,
    dobavljac_pib       TEXT,
    dobavljac_mesto     TEXT,
    osnovica_opsta      REAL    NOT NULL DEFAULT 0,
    pdv_opsta           REAL    NOT NULL DEFAULT 0, -- PDV koji se može odbiti (opšta)
    osnovica_posebna    REAL    NOT NULL DEFAULT 0,
    pdv_posebna         REAL    NOT NULL DEFAULT 0, -- PDV koji se može odbiti (posebna)
    pdv_bez_odbitka     REAL    NOT NULL DEFAULT 0, -- PDV bez prava na odbitak
    osloboden_nabavka   REAL    NOT NULL DEFAULT 0, -- nabavka bez PDV / oslobođena
    ukupno              REAL    NOT NULL DEFAULT 0,
    napomena            TEXT,
    datum_unosa         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
