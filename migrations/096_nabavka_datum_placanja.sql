-- Datum plaćanja računa dobavljača — relevantno za gotovinski PDV odbitak
-- (KPR polje datum_placanja se do sada nikad nije popunjavalo za auto-upise iz nabavke).
ALTER TABLE nabavke ADD COLUMN datum_placanja DATE;
