package model

import "time"

// ProdajniNalog predstavlja zaglavlje jedne prodaje
type ProdajniNalog struct {
	ID                int64
	KlijentID         *int64
	BrojNaloga        string
	Napomena          string
	Ukupno            float64
	NacinPlacanja     string
	Stornirano        bool
	RazlogStorniranja string
	Datum             time.Time
}

// StavkaProdaje predstavlja jednu liniju (artikal) unutar prodaje
type StavkaProdaje struct {
	ID           int64
	NalogID      int64
	ArtikalID    int64
	Kolicina     int
	CenaPoKomadu float64
	Ukupno       float64
	PdvStopa     float64
	PdvIznos     float64
	CenaBezPdv   float64
}

// ProdajniNalogSaDetaljem je nalog sa nazivom klijenta — za prikaz u listi
type ProdajniNalogSaDetaljem struct {
	ProdajniNalog
	KlijentNaziv string
}

// StavkaProdajeSaArtiklom je stavka prodaje sa nazivom artikla — za prikaz u detaljima
type StavkaProdajeSaArtiklom struct {
	StavkaProdaje
	ArtikalNaziv string
	JedinicaMere string
}
