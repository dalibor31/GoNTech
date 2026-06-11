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

// FlashPoruka je jednokratna poruka koja se prikazuje korisniku nakon redirecta
type FlashPoruka struct {
	Tip    string // "uspeh" ili "greska"
	Poruka string
}

// PodaciStranice su zajednički podaci koje svaka stranica prima
type PodaciStranice struct {
	Stranica       string
	NaslovStranice string
	Tema           string
	NazivFirme     string
	Podnazlov      string
	LogoTip        string // "sa_nazivom", "bez_naziva", "slika"
	LogoPutanja    string // putanja do slike, koristi se samo kada je LogoTip "slika"
	Korisnik       string
	KorisnikIme    string // korisničko ime prijavljenog korisnika
	KorisnikUloga  string          // uloga: "superadmin", "admin", "radnik"
	CsrfToken      string          // CSRF zaštitni token za forme
	AssetV         string          // verzija statičkih fajlova (cache-busting za CSS/JS)
	Dozvole        map[string]bool // mapa akcija → dozvoljeno/nije
	Flash          *FlashPoruka    // jednokratna poruka nakon redirecta
	// app pozadina — popunjava se iz podešavanja za sve stranice
	AppPozadina             string
	AppPozadinaOpacity      string // vrednost 0-80 (% overlay zatamnjivanja)
	AppPozadinaBlur         string // vrednost 0-20 (px backdrop-filter blur na elementima)
	AppPozadinaBlurPozadine string // vrednost 0-20 (px filter blur na pozadinskoj slici)
	AppPozadinaGlassOpacity string // vrednost 0-80 (% zatamnjivanje glass elemenata) — samo za ličnu pozadinu
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
	FlashGreska       string
}
