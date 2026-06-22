package model

import (
	"fmt"
	"time"
)

// Statusi servisnog naloga
const (
	StatusPrimljeno  = "Primljeno"
	StatusCekaDelove = "Čeka delove"
	StatusUPopravci  = "U popravci"
	StatusZavrseno   = "Završeno"
	StatusPreuzeto   = "Preuzeto"
)

// SviStatusi je uređena lista statusa za prikaz u dropdownu
var SviStatusi = []string{
	StatusPrimljeno,
	StatusCekaDelove,
	StatusUPopravci,
	StatusZavrseno,
	StatusPreuzeto,
}

// ServisniNalog predstavlja jedan servisni nalog
type ServisniNalog struct {
	ID             int64
	KlijentID      *int64
	TehnicarID     *int64
	BrojNaloga     string
	Uredjaj        string
	SerijskiBroj   string
	OpisKvara      string
	Status         string
	CenaOd         *float64
	CenaDo         *float64
	CenaKonacna    *float64
	Avans          *float64
	Napomena       string
	GarancijaDo    *time.Time
	DatumPrijema   time.Time
	DatumZavrsetka *time.Time
	Ostecenja      string
	PinUredjaja    string
	Pribor         string
	JavniToken     string
}

// ServisniDeo predstavlja jedan artikal ugrađen u servisni nalog
type ServisniDeo struct {
	ID         int64
	NalogID    int64
	ArtikalID  int64
	Kolicina   int
	CenaKomada float64
	Datum      time.Time
}

// Ukupno vraća ukupnu vrednost dela (kolicina × cena)
func (d ServisniDeo) Ukupno() float64 {
	return float64(d.Kolicina) * d.CenaKomada
}

// ServisniDeoSaArtiklom je servisni deo sa nazivom artikla — za prikaz
type ServisniDeoSaArtiklom struct {
	ServisniDeo
	ArtikalNaziv string
	Potrazivano  int // komada koji nedostaju (iz servisni_potrazivani_delovi), 0 ako nema
}

// ServisniPotrazivaniDeo beleži artikle koji nedostaju za servisni nalog —
// količina koja se traži a nije na stanju; ne skida se sa lagera dok ne stigne
type ServisniPotrazivaniDeo struct {
	ID         int64
	NalogID    int64
	ArtikalID  int64
	Kolicina   int
	CenaKomada float64
	Datum      time.Time
	// za prikaz:
	ArtikalNaziv string
}

// ServisniRad predstavlja jednu stavku rada (uslugu) na servisnom nalogu.
// Naziv i cena su snapshot iz cenovnika usluga; cena se može menjati po nalogu.
type ServisniRad struct {
	ID         int64
	NalogID    int64
	UslugaID   int64 // 0 ako je usluga u međuvremenu obrisana iz cenovnika
	Naziv      string
	Kolicina   float64
	CenaKomada float64
	Datum      string
}

// Ukupno vraća ukupnu vrednost rada (kolicina × cena)
func (r ServisniRad) Ukupno() float64 {
	return r.Kolicina * r.CenaKomada
}

// ServisniNalogSaKlijentom proširuje ServisniNalog sa nazivom klijenta za prikaz u listi
type ServisniNalogSaKlijentom struct {
	ServisniNalog
	KlijentNaziv string
}

// KlijentIDVrednost vraća vrednost KlijentID pointera, ili 0 ako je nil
func (n ServisniNalog) KlijentIDVrednost() int64 {
	if n.KlijentID == nil {
		return 0
	}
	return *n.KlijentID
}

// TehnicarIDVrednost vraća vrednost TehnicarID pointera, ili 0 ako je nil
func (n ServisniNalog) TehnicarIDVrednost() int64 {
	if n.TehnicarID == nil {
		return 0
	}
	return *n.TehnicarID
}

// CenaOdStr vraća formatiranu procenu od, ili prazan string ako nije uneta
func (n ServisniNalog) CenaOdStr() string {
	if n.CenaOd == nil {
		return ""
	}
	return fmt.Sprintf("%.2f", *n.CenaOd)
}

// CenaDoStr vraća formatiranu procenu do, ili prazan string ako nije uneta
func (n ServisniNalog) CenaDoStr() string {
	if n.CenaDo == nil {
		return ""
	}
	return fmt.Sprintf("%.2f", *n.CenaDo)
}

// CenaKonacnaStr vraća formatiranu konačnu cenu, ili prazan string ako nije uneta
func (n ServisniNalog) CenaKonacnaStr() string {
	if n.CenaKonacna == nil {
		return ""
	}
	return fmt.Sprintf("%.2f", *n.CenaKonacna)
}

// AvansStr vraća formatirani avans, ili prazan string ako nije unet
func (n ServisniNalog) AvansStr() string {
	if n.Avans == nil {
		return ""
	}
	return fmt.Sprintf("%.2f", *n.Avans)
}

// PreostaloZaNaplatu vraća razliku konacna_cena − avans, minimum 0.
// Vraća nil ako konačna cena nije uneta.
func (n ServisniNalog) PreostaloZaNaplatu() *float64 {
	if n.CenaKonacna == nil {
		return nil
	}
	avans := 0.0
	if n.Avans != nil {
		avans = *n.Avans
	}
	v := *n.CenaKonacna - avans
	if v < 0 {
		v = 0
	}
	return &v
}

// PreostaloZaNaplatuStr vraća formatirano preostalo za naplatu, ili prazan string
func (n ServisniNalog) PreostaloZaNaplatuStr() string {
	v := n.PreostaloZaNaplatu()
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%.2f", *v)
}
