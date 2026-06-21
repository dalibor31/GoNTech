-- Dodaje tip identifikacionog broja klijenta (fizičko lice): 'jmbg' ili 'licna_karta'.
-- Vrednost broja se i dalje čuva u koloni jmbg; ova kolona govori šta je uneto.
-- Postojeći klijenti podrazumevano dobijaju 'jmbg' jer je to do sada i bio JMBG.
ALTER TABLE klijenti ADD COLUMN tip_identifikacije TEXT NOT NULL DEFAULT 'jmbg';
