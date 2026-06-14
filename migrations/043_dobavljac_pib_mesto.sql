-- Dodaje PIB i mesto/grad u dobavljače. PIB dobavljača je obavezan podatak za
-- knjigu primljenih računa (KPR) i POPDV (evidencija prethodnog/odbitnog PDV-a).
ALTER TABLE dobavljaci ADD COLUMN pib TEXT;
ALTER TABLE dobavljaci ADD COLUMN mesto TEXT;
