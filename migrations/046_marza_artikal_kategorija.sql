-- Dodaje podrazumevanu maržu (%) na artikle i kategorije za kalkulaciju
-- prodajne cene pri nabavci. Kolone su NULL kada marža nije postavljena —
-- tako se „nije postavljeno" razlikuje od svesno unete marže 0%.
-- Redosled izvođenja marže: artikal.marza → kategorija.marza → globalna kalkulacija_marza.
ALTER TABLE artikli ADD COLUMN marza REAL;
ALTER TABLE kategorije ADD COLUMN marza REAL;
