package model

import "time"

// Tipovi artikla. Proizvod prati stanje na lageru; usluga i trošak ga ne prate.
const (
	TipProizvod = "proizvod"
	TipUsluga   = "usluga"
	TipTrosak   = "trosak"
)

// Artikal predstavlja jedan artikal u magacinu
type Artikal struct {
	ID           int64
	KategorijaID *int64
	Sifra        string
	Barkod       string
	Naziv        string
	Opis         string
	Tip          string // proizvod | usluga | trosak
	JedinicaMere string // kom, sat, set, m, l, kg ...
	Kolicina     int
	KolicinMin   int
	Lokacija     string
	NabavnaCena  float64
	ProdajnaCena float64
	PdvStopa     float64
	Marza        *float64 // podrazumevana marža (%) za kalkulaciju; NULL = nije postavljeno
	Napomena     string
	DatumUnosa   time.Time
	Arhiviran    bool // artikal u prometu koji je sklonjen iz aktivne liste; istorija ostaje
}

// PratiLager vraća true samo za proizvode (usluge i troškovi nemaju stanje na lageru).
// Prazan tip se tretira kao proizvod radi kompatibilnosti sa starim zapisima.
func (a Artikal) PratiLager() bool {
	return a.Tip == TipProizvod || a.Tip == ""
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
	Kod   string   // prefiks za šifru artikla (npr. KOMP -> KOMP-0001)
	Marza *float64 // podrazumevana marža (%) za artikle ove kategorije; NULL = nije postavljeno
}

// ArtikalSaKategorijom je artikal sa nazivom kategorije — za prikaz u tabeli
type ArtikalSaKategorijom struct {
	Artikal
	KategorijaNaziv string
	KategorijaMarza *float64 // marža kategorije; za fallback predloga marže pri nabavci
	KriticnaZaliha  bool
}
