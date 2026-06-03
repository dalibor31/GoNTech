package model

// StavkaServisa prikazuje jedan servisni nalog na dashboardu
type StavkaServisa struct {
	Uredjaj      string
	Status       string
	BojaTacke    string
	DatumPrijema string // kratki format, npr. "01.06."
}

// StavkaZalihe prikazuje jedan artikal sa kritičnom zalihom
type StavkaZalihe struct {
	Naziv     string
	Kolicina  int
	BojaTacke string
}

// StavkaProdajePregled prikazuje jedan prodajni nalog na dashboardu
type StavkaProdajePregled struct {
	BrojNaloga   string
	KlijentNaziv string
	Ukupno       float64
	Datum        string // kratki format, npr. "01.06."
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
	KorisnikIme    string // korisničko ime prijavljenog korisnika
	KorisnikUloga  string // uloga: "superadmin", "admin", "radnik"
}

// PodaciDashboarda su podaci specifični za dashboard stranicu
type PodaciDashboarda struct {
	PodaciStranice
	BrojArtikala      int
	AktivniServisi    int
	PrihodOvogMeseca  float64
	KriticnaZaliha    int
	AktivniPodsetnici int
	PoslednjiServisi  []StavkaServisa
	KriticneZalihe    []StavkaZalihe
	PoslednjeProdaje  []StavkaProdajePregled
}
