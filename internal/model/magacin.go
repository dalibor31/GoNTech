package model

import "time"

// Tipovi magacinskih promena
const (
	PromenaUlazNabavka  = "ulaz_nabavka"
	PromenaIzlazProdaja = "izlaz_prodaja"
	PromenaIzlazServis  = "izlaz_servis"
	PromenaPovracaj     = "povracaj"
	PromenaKorekcija    = "korekcija"
)

// MagacinskaPromenaSaDetaljem je promena stanja artikla sa nazivom artikla
type MagacinskaPromenaSaDetaljem struct {
	ID              int64
	ArtikalID       int64
	ArtikalNaziv    string
	TipPromene      string
	ReferentniID    int64
	PromenaKolicine int
	StanjePre       int
	StanjePosle     int
	KorisnikID      *int64
	Napomena        string
	Datum           time.Time
}
