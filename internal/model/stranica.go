package model

// StavkaServisa prikazuje jedan servisni nalog na dashboardu
type StavkaServisa struct {
	Uredjaj   string
	Status    string
	BojaTacke string
}

// StavkaZalihe prikazuje jedan artikal sa kritičnom zalihom
type StavkaZalihe struct {
	Naziv     string
	Kolicina  int
	BojaTacke string
}

// PodaciStranice su zajednički podaci koje svaka stranica prima
type PodaciStranice struct {
	Stranica       string
	NaslovStranice string
	Tema           string
	NazivFirme     string
	Podnazlov      string
	LogoTip        string // "ikonica", "tekst", "slika"
	LogoPutanja    string // putanja do slike, koristi se samo kada je LogoTip "slika"
	Korisnik       string
}

// PodaciDashboarda su podaci specifični za dashboard stranicu
type PodaciDashboarda struct {
	PodaciStranice
	BrojArtikala      int
	AktivniServisi    int
	ProdajaOvogMeseca int
	KriticnaZaliha    int
	PoslednjiServisi  []StavkaServisa
	KriticneZalihe    []StavkaZalihe
}
