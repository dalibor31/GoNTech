package model

import "time"

// Artikal predstavlja jedan artikal u magacinu
type Artikal struct {
	ID           int64
	KategorijaID *int64
	Sifra        string
	Barkod       string
	Naziv        string
	Opis         string
	Kolicina     int
	KolicinMin   int
	Lokacija     string
	NabavnaCena  float64
	ProdajnaCena float64
	PdvStopa     float64
	Marza        *float64 // podrazumevana marža (%) za kalkulaciju; NULL = nije postavljeno
	Napomena     string
	DatumUnosa   time.Time
}

// CenaBezPdv izračunava prodajnu cenu bez PDV-a
func (a Artikal) CenaBezPdv() float64 {
	return a.ProdajnaCena / (1 + a.PdvStopa/100)
}

// PdvIznos izračunava iznos PDV-a za jednu jedinicu
func (a Artikal) PdvIznos() float64 {
	return a.ProdajnaCena - a.CenaBezPdv()
}

// Kategorija predstavlja kategoriju artikala
type Kategorija struct {
	ID    int64
	Naziv string
	Opis  string
	Marza *float64 // podrazumevana marža (%) za artikle ove kategorije; NULL = nije postavljeno
}

// ArtikalSaKategorijom je artikal sa nazivom kategorije — za prikaz u tabeli
type ArtikalSaKategorijom struct {
	Artikal
	KategorijaNaziv string
	KategorijaMarza *float64 // marža kategorije; za fallback predloga marže pri nabavci
	KriticnaZaliha  bool
}
