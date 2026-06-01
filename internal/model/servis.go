package model

import "time"

// Statusi servisnog naloga
const (
	StatusPrimljeno    = "Primljeno"
	StatusDijagnostika = "U dijagnostici"
	StatusCekaDelove   = "Čeka delove"
	StatusUPopravci    = "U popravci"
	StatusZavrseno     = "Završeno"
	StatusPreuzeto     = "Preuzeto"
)

// SviStatusi je uređena lista statusa za prikaz u dropdownu
var SviStatusi = []string{
	StatusPrimljeno,
	StatusDijagnostika,
	StatusCekaDelove,
	StatusUPopravci,
	StatusZavrseno,
	StatusPreuzeto,
}

// ServisniNalog predstavlja jedan servisni nalog
type ServisniNalog struct {
	ID             int64
	KlijentID      *int64
	BrojNaloga     string
	Uredjaj        string
	SerijskiBroj   string
	OpisKvara      string
	Status         string
	CenaOd         *float64
	CenaDo         *float64
	CenaKonacna    *float64
	Avans          *float64
	Napomena       string
	DatumPrijema   time.Time
	DatumZavrsetka *time.Time
}

// ServisniNalogSaKlijentom proširuje ServisniNalog sa nazivom klijenta za prikaz u listi
type ServisniNalogSaKlijentom struct {
	ServisniNalog
	KlijentNaziv string
}
