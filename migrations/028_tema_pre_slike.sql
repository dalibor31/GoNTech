INSERT INTO podesavanja (kljuc, vrednost)
VALUES ('tema_pre_slike', '')
ON CONFLICT(kljuc) DO NOTHING;
