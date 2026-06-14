package model

import (
	"strings"
	"time"
)

// Klijent predstavlja jednog klijenta — fizičko lice ili firmu
type Klijent struct {
	ID         int64
	Tip        string
	Ime        string
	Prezime    string
	JMBG       string
	NazivFirme string
	PIB        string
	Telefon    string
	Email      string
	Mesto      string
	Napomena   string
	DatumUnosa time.Time
}

// PunoIme vraća ime i prezime za fizičko lice, ili naziv firme za pravno
func (k Klijent) PunoIme() string {
	if k.Tip == "pravno" {
		return k.NazivFirme
	}
	return strings.TrimSpace(k.Ime + " " + k.Prezime)
}
