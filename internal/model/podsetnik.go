package model

import "time"

// TipOpsti je jedini trenutno podržani tip podsetnika
const TipOpsti = "opsti"

// Podsetnik predstavlja jedan podsetnik
type Podsetnik struct {
	ID               int64
	Naslov           string
	Napomena         string
	DatumPodsecanja  time.Time
	Zavrseno         bool
	Tip              string
	DatumUnosa       time.Time
}

// JePrekoracen vraća true ako datum podsećanja je prošao a podsetnik nije završen
func (p Podsetnik) JePrekoracen() bool {
	return !p.Zavrseno && p.DatumPodsecanja.Before(time.Now())
}
