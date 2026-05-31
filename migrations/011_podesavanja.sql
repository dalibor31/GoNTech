CREATE TABLE IF NOT EXISTS podesavanja (
    kljuc   TEXT PRIMARY KEY,
    vrednost TEXT NOT NULL
);

-- podrazumevane vrednosti
INSERT OR IGNORE INTO podesavanja (kljuc, vrednost) VALUES
    ('naziv_firme',  'NTech'),
    ('podnazlov',    'Servis računara'),
    ('logo_tip',     'ikonica'),
    ('logo_putanja', ''),
    ('tema',         'tamna');
