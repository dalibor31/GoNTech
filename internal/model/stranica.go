package model

// StavkaServisa prikazuje jedan servisni nalog na dashboardu
type StavkaServisa struct {
	Uredjaj   string
	Status    string
	BojaTacke string // "#16a34a" zelena, "#f59e0b" žuta, "#dc2626" crvena
}

// StavkaZalihe prikazuje jedan artikal sa kritičnom zalihom
type StavkaZalihe struct {
	Naziv     string
	Kolicina  int
	BojaTacke string
}

// PodaciStranice su zajednički podaci koje svaka stranica prima
type PodaciStranice struct {
	Stranica       string // naziv aktivne stranice za sidebar
	NaslovStranice string // naslov u topbaru
	Tema           string // aktivna tema: "tamna", "svetla", "zelena", "ljubicasta"
	NazivFirme     string // naziv firme za logo zonu
	Logo           string // putanja do logo fajla, opciono
	Korisnik       string // ime korisnika za avatar
}

// PodaciDashboarda su podaci specifični za dashboard stranicu
type PodaciDashboarda struct {
	PodaciStranice
	BrojArtikala       int
	AktivniServisi     int
	ProdajaOvogMeseca  int
	KriticnaZaliha     int
	PoslednjiServisi   []StavkaServisa
	KriticneZalihe     []StavkaZalihe
}
