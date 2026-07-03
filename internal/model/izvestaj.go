package model

import "time"

// Tipovi u ovom fajlu su SIROVI redovi koje vraća IzvestajRepository.
// Prezentaciju (formatiranje datuma, boje, rang, sklapanje niza meseci) radi
// handler — ovde su samo podaci iz baze.

// ServisRedDashboard je jedan red za listu poslednjih servisa na dashboardu
type ServisRedDashboard struct {
	ID           int64
	Uredjaj      string
	Status       string
	DatumPrijema time.Time
}

// ZalihaRed je artikal sa kritičnom zalihom (naziv + količina)
type ZalihaRed struct {
	Naziv       string
	Kolicina    int
	KolicinaMin int
}

// ProdajaRedDashboard je jedan red za listu poslednjih prodaja na dashboardu
type ProdajaRedDashboard struct {
	BrojNaloga   string
	Ukupno       float64
	Datum        time.Time
	KlijentNaziv string
}

// MesecniIznos je zbir za jedan mesec (ključ je "YYYY-MM")
type MesecniIznos struct {
	Mesec string
	Iznos float64
}

// StariNalogRed je otvoreni servisni nalog stariji od praga (sirov datum)
type StariNalogRed struct {
	ID           int64
	BrojNaloga   string
	Uredjaj      string
	Status       string
	KlijentNaziv string
	DatumPrijema time.Time
}

// TopArtikalRed je artikal rangiran po prodatoj količini (bez ranga — dodaje handler)
type TopArtikalRed struct {
	Naziv          string
	Kategorija     string
	UkupnoKolicina int
	UkupnoPrihod   float64
}

// TopKlijentRed je klijent rangiran po ukupnoj vrednosti naloga (bez ranga)
type TopKlijentRed struct {
	Naziv          string
	UkupnoVrednost float64
	BrojNaloga     int
}

// PrometniRed je jedan red prometnog lista magacina
type PrometniRed struct {
	Datum           time.Time
	ArtikalNaziv    string
	ArtikalSifra    string
	TipPromene      string
	PromenaKolicine int
	StanjePre       int
	StanjePosle     int
	Napomena        string
}

// StanjeZalihaRed je jedan red izveštaja o stanju zaliha
type StanjeZalihaRed struct {
	Naziv          string
	Sifra          string
	Kategorija     string
	Kolicina       int
	KolicinaMin    int
	NabavnaCena    float64
	ProdajnaCena   float64
	CenaSaPdv      float64
	VrednostZalihe float64 // kolicina × nabavna_cena
	VrednostSaPdv  float64 // kolicina × cena_sa_pdv
}
