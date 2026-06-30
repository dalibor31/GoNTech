package model

import "time"

// KpoZapis je jedan red u knjizi o ostvarenom prometu.
type KpoZapis struct {
	ID            int64
	DatumPrometa  time.Time
	RedniBroj     *int
	BrojDokumenta string
	Opis          string
	Prihod        float64
	NacinPlacanja string
	Napomena      string
	Izvor         string // "rucno" | "prodaja" | "servis"
	IzvorID       *int64
	DatumUnosa    time.Time
}

// KpoSume su zbrojeni prihodi za period.
type KpoSume struct {
	Prihod float64
	Broj   int
}
