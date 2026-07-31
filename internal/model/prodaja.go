package model

import "time"

// ProdajniNalog predstavlja zaglavlje jedne prodaje
type ProdajniNalog struct {
	ID                int64
	KlijentID         *int64
	BrojNaloga        string
	Napomena          string
	Ukupno            float64
	NacinPlacanja     string
	Stornirano        bool
	RazlogStorniranja string
	Datum             time.Time
	// IdempotencyKey je UUID koji frontend generiše po otvaranju forme (skriveno polje).
	// Ako isti ključ već postoji u bazi, Kreiraj ne pravi novi nalog nego vraća postojeći —
	// štiti od duplog POST-a (dupli klik, "Nazad" pa ponovni submit, mrežni retry, dva taba).
	// Prazan string znači da pozivalac ne koristi zaštitu (npr. testovi, budući pozivaoci).
	IdempotencyKey string
}

// StavkaProdaje predstavlja jednu liniju (artikal) unutar prodaje
type StavkaProdaje struct {
	ID             int64
	NalogID        int64
	ArtikalID      int64
	Kolicina       int
	CenaPoKomadu   float64
	PopustProcenat float64 // rabat u procentima (npr. 10 = 10% popusta)
	Ukupno         float64
	PdvStopa       float64
	PdvIznos       float64
	CenaBezPdv     float64
}

// UkupnoBezPdv vraća ukupno bez PDV-a za stavku (CenaBezPdv × količina)
func (s StavkaProdaje) UkupnoBezPdv() float64 {
	return s.CenaBezPdv * float64(s.Kolicina)
}

// ProdajniNalogSaDetaljem je nalog sa nazivom klijenta — za prikaz u listi
type ProdajniNalogSaDetaljem struct {
	ProdajniNalog
	KlijentNaziv string
}

// StavkaProdajeSaArtiklom je stavka prodaje sa nazivom artikla — za prikaz u detaljima
type StavkaProdajeSaArtiklom struct {
	StavkaProdaje
	ArtikalNaziv    string
	JedinicaMere    string
	KategorijaNaziv string
}

// DnevniPrometKir su zbrojeni iznosi maloprodaje za jedan dan, razvrstani po PDV stopi.
// Koristi se za kreiranje zbirnog KIR zapisa (promet fizičkim licima).
type DnevniPrometKir struct {
	OsnovicaOpsta   float64 // neto suma po stopi 20%
	PdvOpsta        float64
	OsnovicaPosebna float64 // neto suma po stopi 10%
	PdvPosebna      float64
	BrojNaloga      int // ukupan broj prodajnih naloga taj dan
}
