-- Tri polja za pravilno popunjavanje KPR (Pravilnik o PDV evidencijama):
-- broj_racuna: broj dokumenta sa računa dobavljača (umesto sintetičkog NAB-{id})
-- datum_racuna: datum prometa sa računa dobavljača (umesto datuma unosa)
-- pdv_iznos: stvarni PDV sa računa dobavljača (umesto aproksimacije iz stope)
ALTER TABLE nabavke ADD COLUMN broj_racuna TEXT;
ALTER TABLE nabavke ADD COLUMN datum_racuna DATE;
ALTER TABLE nabavke ADD COLUMN pdv_iznos REAL NOT NULL DEFAULT 0;
