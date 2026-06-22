package model

import "time"

// TipOpsti je jedini trenutno podržani tip podsetnika
const TipOpsti = "opsti"

// Podsetnik predstavlja jedan podsetnik
type Podsetnik struct {
	ID              int64
	Naslov          string
	Napomena        string
	DatumPodsecanja time.Time
	Zavrseno        bool
	Tip             string
	DatumUnosa      time.Time
	KorisnikID      *int64 // ako nil — podsetnik nije dodeljen konkretnom korisniku
}

// JePrekoracen vraća true ako datum podsećanja je prošao a podsetnik nije završen
func (p Podsetnik) JePrekoracen() bool {
	return !p.Zavrseno && p.DatumPodsecanja.Before(time.Now())
}

// KorisnikIDVal vraća vrednost KorisnikID pokazivača ili 0 ako je nil — za poređenje u šablonima
func (p Podsetnik) KorisnikIDVal() int64 {
	if p.KorisnikID == nil {
		return 0
	}
	return *p.KorisnikID
}
