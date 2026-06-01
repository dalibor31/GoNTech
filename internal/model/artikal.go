package model

import "time"

// Artikal predstavlja jedan artikal u magacinu
type Artikal struct {
	ID           int64
	KategorijaID *int64
	Naziv        string
	Opis         string
	Kolicina     int
	KolicinMin   int
	Lokacija     string
	NabavnaCena  float64
	ProdajnaCena float64
	Napomena     string
	DatumUnosa   time.Time
}

// Kategorija predstavlja kategoriju artikala
type Kategorija struct {
	ID    int64
	Naziv string
	Opis  string
}

// ArtikalSaKategorijom je artikal sa nazivom kategorije — za prikaz u tabeli
type ArtikalSaKategorijom struct {
	Artikal
	KategorijaNaziv string
	KriticnaZaliha  bool // true ako je kolicina <= kolicina_min
}
