package model

import "time"

// Klijent predstavlja jednog klijenta — fizičko lice ili firmu
type Klijent struct {
	ID         int64
	Ime        string
	Prezime    string
	NazivFirme string
	PIB        string
	Telefon    string
	Email      string
	Napomena   string
	DatumUnosa time.Time
}
