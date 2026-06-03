package model

import (
	"fmt"
	"time"
)

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

// KlijentIDVrednost vraća vrednost KlijentID pointera, ili 0 ako je nil
func (n ServisniNalog) KlijentIDVrednost() int64 {
	if n.KlijentID == nil {
		return 0
	}
	return *n.KlijentID
}

// CenaOdStr vraća formatiranu procenu od, ili prazan string ako nije uneta
func (n ServisniNalog) CenaOdStr() string {
	if n.CenaOd == nil {
		return ""
	}
	return fmt.Sprintf("%.2f", *n.CenaOd)
}

// CenaDoStr vraća formatiranu procenu do, ili prazan string ako nije uneta
func (n ServisniNalog) CenaDoStr() string {
	if n.CenaDo == nil {
		return ""
	}
	return fmt.Sprintf("%.2f", *n.CenaDo)
}

// CenaKonacnaStr vraća formatiranu konačnu cenu, ili prazan string ako nije uneta
func (n ServisniNalog) CenaKonacnaStr() string {
	if n.CenaKonacna == nil {
		return ""
	}
	return fmt.Sprintf("%.2f", *n.CenaKonacna)
}

// AvansStr vraća formatirani avans, ili prazan string ako nije unet
func (n ServisniNalog) AvansStr() string {
	if n.Avans == nil {
		return ""
	}
	return fmt.Sprintf("%.2f", *n.Avans)
}
