package model

import (
	"math"
	"time"
)

// Nabavka predstavlja zaglavlje jedne nabavke
type Nabavka struct {
	ID                int64
	DobavljacID       *int64
	Napomena          string
	Ukupno            float64
	MetodRaspodele    string // "vrednost" ili "kolicina"; prazno = nema zavisnih troškova
	Stornirano        bool
	RazlogStorniranja string
	Datum             time.Time
	BrojRacuna        string     // broj računa dobavljača (za KPR BrojDokumenta)
	DatumRacuna       *time.Time // datum prometa sa računa dobavljača (za KPR DatumPrometa)
	PdvIznos          float64    // stvarni PDV sa računa; 0 = koristiti aproksimaciju iz stope
}

// NabavkaTrosak je jedna stavka zavisnih troškova nabavke (npr. prevoz, carina)
type NabavkaTrosak struct {
	ID        int64
	NabavkaID int64
	Naziv     string
	Iznos     float64
}

// StavkaNabavke predstavlja jednu liniju (artikal) unutar nabavke
type StavkaNabavke struct {
	ID           int64
	NabavkaID    int64
	ArtikalID    int64
	Kolicina     int
	CenaPoKomadu float64
	Ukupno       float64
}

// RasporediTroskove raspodeljuje ukupan zavisni trošak na stavke nabavke i vraća
// kalkulativnu nabavnu cenu po komadu za svaku stavku (isti redosled kao ulazne stavke).
// metod "kolicina" deli trošak po broju komada; svaka druga vrednost (uklj. "vrednost")
// deli po nabavnoj vrednosti stavke (količina × cena).
// Ako nema troška ili je osnovica nula, vraća nepromenjenu cenu po komadu.
func RasporediTroskove(stavke []StavkaNabavke, ukupanTrosak float64, metod string) []float64 {
	kalk := make([]float64, len(stavke))

	// osnovica raspodele po izabranom metodu
	var osnovica float64
	for _, s := range stavke {
		if metod == "kolicina" {
			osnovica += float64(s.Kolicina)
		} else {
			osnovica += float64(s.Kolicina) * s.CenaPoKomadu
		}
	}

	for i, s := range stavke {
		kalk[i] = s.CenaPoKomadu
		// bez troška, bez osnovice ili bez količine — nema šta da se raspodeli
		if ukupanTrosak <= 0 || osnovica <= 0 || s.Kolicina == 0 {
			continue
		}
		var udeo float64
		if metod == "kolicina" {
			udeo = float64(s.Kolicina) / osnovica
		} else {
			udeo = (float64(s.Kolicina) * s.CenaPoKomadu) / osnovica
		}
		trosakPoKomadu := (ukupanTrosak * udeo) / float64(s.Kolicina)
		// zaokruženo na 2 decimale — kalkulativna nabavna je cena koja se čuva
		kalk[i] = math.Round((s.CenaPoKomadu+trosakPoKomadu)*100) / 100
	}

	return kalk
}

// NabavkaSaDetaljem je nabavka sa nazivom dobavljača — za prikaz u listi
type NabavkaSaDetaljem struct {
	Nabavka
	DobavljacNaziv string
}

// StavkaSaArtiklom je stavka nabavke sa nazivom artikla — za prikaz u formi i detaljima
type StavkaSaArtiklom struct {
	StavkaNabavke
	ArtikalNaziv string
}
