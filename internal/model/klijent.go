package model

import (
	"strings"
	"time"
)

// Klijent predstavlja jednog klijenta — fizičko lice ili firmu
type Klijent struct {
	ID      int64
	Tip     string
	Ime     string
	Prezime string
	JMBG    string // vrednost identifikacionog broja (JMBG ili broj lične karte)
	// TipIdentifikacije govori šta je u polju JMBG: "jmbg" ili "licna_karta".
	// Validnost vrednosti (JMBG = 13 cifara, format broja lične karte) proverava se kasnije.
	TipIdentifikacije string
	NazivFirme        string
	PIB               string
	Telefon           string
	Email             string
	Mesto             string
	Napomena          string
	DatumUnosa        time.Time
}

// PunoIme vraća ime i prezime za fizičko lice, ili naziv firme za pravno
func (k Klijent) PunoIme() string {
	if k.Tip == "pravno" {
		return k.NazivFirme
	}
	return strings.TrimSpace(k.Ime + " " + k.Prezime)
}

// OznakaIdentifikacije vraća labelu za prikaz: „Br. lične karte" ili „JMBG" (podrazumevano)
func (k Klijent) OznakaIdentifikacije() string {
	if k.TipIdentifikacije == "licna_karta" {
		return "Br. lične karte"
	}
	return "JMBG"
}
