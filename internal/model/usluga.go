package model

import "time"

// Usluga je stavka cenovnika usluga (npr. čišćenje laptopa, zamena ventilatora).
// Za razliku od artikla, usluga ne prati lager i nema nabavnu cenu ni dobavljače —
// ima samo cenu usluge i PDV stopu. Kategorija je tekstualna oznaka.
type Usluga struct {
	ID           int64
	Sifra        string
	Naziv        string
	Kategorija   string
	JedinicaMere string
	Cena         float64
	PdvStopa     float64
	Opis         string
	Arhiviran    bool
	DatumUnosa   time.Time
}

func (u Usluga) CenaSaPdv() float64 {
	return u.Cena * (1 + u.PdvStopa/100)
}
