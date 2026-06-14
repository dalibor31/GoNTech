package model

import "time"

// Nivelacija je zapis o promeni prodajne cene artikla (revizioni trag).
// Razlika i procenat se izvode iz stare i nove cene (ne čuvaju se u bazi).
type Nivelacija struct {
	ID           int64
	ArtikalID    int64
	ArtikalNaziv string // iz JOIN-a, radi prikaza; nije kolona u nivelacije
	StaraCena    float64
	NovaCena     float64
	Razlog       string
	Izvor        string // "rucno" | "izmena" | "kalkulacija"
	KorisnikID   *int64
	KorisnikIme  string // iz JOIN-a, radi prikaza
	Datum        time.Time
	DatumUnosa   time.Time
}

// Razlika vraća apsolutnu promenu cene (nova − stara); negativna znači pojeftinjenje.
func (n Nivelacija) Razlika() float64 {
	return n.NovaCena - n.StaraCena
}

// Procenat vraća procentualnu promenu u odnosu na staru cenu (0 ako je stara cena 0).
func (n Nivelacija) Procenat() float64 {
	if n.StaraCena == 0 {
		return 0
	}
	return (n.NovaCena - n.StaraCena) / n.StaraCena * 100
}

// Poskupljenje vraća true kada je nova cena veća od stare.
func (n Nivelacija) Poskupljenje() bool {
	return n.NovaCena > n.StaraCena
}
