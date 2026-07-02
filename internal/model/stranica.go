package model

// StavkaServisa prikazuje jedan servisni nalog na dashboardu
type StavkaServisa struct {
	ID           int64
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
	Stranica        string
	NaslovStranice  string
	Tema            string
	NazivFirme      string
	Podnazlov       string
	LogoPutanja     string // putanja do slike loga firme
	TopbarLogoSlika bool   // prikaži logo sliku u topbaru
	AvatarPutanja   string // putanja do lične avatar slike korisnika
	Korisnik        string
	KorisnikIme     string          // korisničko ime prijavljenog korisnika
	KorisnikUloga   string          // uloga: "superadmin", "admin", "radnik"
	CsrfToken       string          // CSRF zaštitni token za forme
	AssetV          string          // verzija statičkih fajlova (cache-busting za CSS/JS)
	Dozvole         map[string]bool // mapa akcija → dozvoljeno/nije
	Moduli          map[string]bool // mapa zakonskih modula → uključen za firmu (profil firme)
	Flash           *FlashPoruka    // jednokratna poruka nakon redirecta
	// app pozadina — popunjava se iz podešavanja za sve stranice
	AppPozadina             string
	AppPozadinaOpacity      string // vrednost 0-80 (% overlay zatamnjivanja)
	AppPozadinaBlur         string // vrednost 0-20 (px backdrop-filter blur na elementima)
	AppPozadinaBlurPozadine string // vrednost 0-20 (px filter blur na pozadinskoj slici)
	AppPozadinaGlassOpacity string // vrednost 0-80 (% zatamnjivanje glass elemenata) — samo za ličnu pozadinu
	LokalnaAnimacija        string // "" | "fadeInUp" | "fadeIn" | "scaleIn" | "slideLeft"
	LokalniHover            string // "" | "bez" | "podizanje" | "svetlost"
	LokalnaBrzinaAnimacije  string // "" | "0.2" | "0.4" | ... | "1.5" (sekunde)
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
	PrazninaKnjige    PrazninaKnjigovodstva
}

// PrazninaKnjigovodstva sažima broj naloga bez KIR/KPO/fiskalnog upisa i broj
// sumnjivih duplikata (naloga koje automatski backfill namerno preskače) — prikazuje
// se kao kartica na dashboard-u.
type PrazninaKnjigovodstva struct {
	BezKirProdaja         int     // B2B prodaje bez KIR upisa
	BezKirServis          int     // B2B servisi bez KIR upisa
	BezKirProdajaID       []int64 // ID-jevi — za direktan link sa dashboard-a
	BezKirServisID        []int64
	BezKpoProdaja         int // prodaje bez KPO upisa
	BezKpoServis          int // servisi bez KPO upisa
	BezKpoProdajaID       []int64
	BezKpoServisID        []int64
	BezFiskalnogProdaja   int // prodajni nalozi bez izdatog fiskalnog računa
	BezFiskalnogServis    int // preuzeti servisni nalozi bez izdatog fiskalnog računa
	BezFiskalnogProdajaID []int64
	BezFiskalnogServisID  []int64
	BezRefundaProdaja     int     // stornirane prodaje čiji fiskalni refund nije uspeo (best-effort poziv pao)
	BezRefundaServis      int     // stornirani servisi čiji fiskalni refund nije uspeo (best-effort poziv pao)
	BezRefundaProdajaID   []int64 // ID-jevi stornirane prodaje bez refunda — za direktan link sa dashboard-a
	BezRefundaServisID    []int64 // ID-jevi storniranih servisa bez refunda — za direktan link sa dashboard-a
	SumnjiviDupli         int
}

// BezKir vraća ukupan broj naloga (prodaja + servis) bez KIR upisa.
func (p PrazninaKnjigovodstva) BezKir() int {
	return p.BezKirProdaja + p.BezKirServis
}

// BezKpo vraća ukupan broj naloga (prodaja + servis) bez KPO upisa.
func (p PrazninaKnjigovodstva) BezKpo() int {
	return p.BezKpoProdaja + p.BezKpoServis
}

// BezFiskalnog vraća ukupan broj naloga (prodaja + servis) bez izdatog fiskalnog
// računa — koristi se za prikaz zbirnog broja na dashboard-u.
func (p PrazninaKnjigovodstva) BezFiskalnog() int {
	return p.BezFiskalnogProdaja + p.BezFiskalnogServis
}

// BezRefunda vraća ukupan broj storniranih naloga (prodaja + servis) čiji fiskalni
// refund nije uspeo — koristi se za prikaz zbirnog broja na dashboard-u.
func (p PrazninaKnjigovodstva) BezRefunda() int {
	return p.BezRefundaProdaja + p.BezRefundaServis
}

// Ukupno vraća zbir svih praznina UKLJUČUJUĆI sumnjive duplikate — koristi se da se
// odluči da li se kartica uopšte prikazuje na dashboard-u (i kad ostanu samo
// duplikati za ručnu proveru, kartica mora da ostane vidljiva).
func (p PrazninaKnjigovodstva) Ukupno() int {
	return p.BezKir() + p.BezKpo() + p.BezFiskalnog() + p.BezRefunda() + p.SumnjiviDupli
}
