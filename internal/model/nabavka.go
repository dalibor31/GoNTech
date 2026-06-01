package model

import "time"

// Nabavka predstavlja zaglavlje jedne nabavke
type Nabavka struct {
	ID          int64
	DobavljacID *int64
	Napomena    string
	Ukupno      float64
	Datum       time.Time
}

// StavkaNabavke predstavlja jednu liniju (artikal) unutar nabavke
type StavkaNabavke struct {
	ID           int64
	NabavkaID    int64
	ArtikalID    int64
	Kolicina     int
	CenaPoKomadu float64
	Ukupno       float64
}

// NabavkaSaDetaljem je nabavka sa nazivom dobavljača — za prikaz u listi
type NabavkaSaDetaljem struct {
	Nabavka
	DobavljacNaziv string
}

// StavkaSaArtiklom je stavka nabavke sa nazivom artikla — za prikaz u formi i detaljima
type StavkaSaArtiklom struct {
	StavkaNabavke
	ArtikalNaziv string
}
