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

// KpoStorno gradi storno zapis za dati KPO red: negiran prihod, upisan pod istim
// izvorom/izvor_id kao original. Knjiga o ostvarenom prometu paušalaca vodi redni
// broj bez prekida (Pravilnik) — original se nikad ne briše, storno se dodaje kao
// poništavajuća stavka koja upućuje na original.
func KpoStorno(original KpoZapis, razlog string, datumStorna time.Time) KpoZapis {
	napomena := "Storno naloga " + original.BrojDokumenta
	if razlog != "" {
		napomena += ": " + razlog
	}
	return KpoZapis{
		DatumPrometa:  datumStorna,
		BrojDokumenta: "STORNO-" + original.BrojDokumenta,
		Opis:          original.Opis,
		Prihod:        -original.Prihod,
		NacinPlacanja: original.NacinPlacanja,
		Napomena:      napomena,
		Izvor:         original.Izvor,
		IzvorID:       original.IzvorID,
	}
}
