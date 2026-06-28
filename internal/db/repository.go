package db

import (
	"context"
	"errors"
	"time"

	"ntech/internal/model"
)

// ErrArtikalUUpotrebi se vraća kad se artikal ne može obrisati jer postoji u prometu
// (prodaja, nabavka, magacinske promene ili servisni nalozi). Tada se artikal arhivira.
var ErrArtikalUUpotrebi = errors.New("ntech: artikal je u upotrebi")

// ArtikalRepository definiše operacije nad artiklima
type ArtikalRepository interface {
	Lista(ctx context.Context, filter ArtikalFilter) ([]model.ArtikalSaKategorijom, error)
	PrebrojiPoFilteru(ctx context.Context, filter ArtikalFilter) (int, error)
	DohvatiID(ctx context.Context, id int64) (*model.Artikal, error)
	Kreiraj(ctx context.Context, a *model.Artikal) (int64, error)
	Izmeni(ctx context.Context, a *model.Artikal) error
	// AzurirajCene menja samo nabavnu i prodajnu cenu (kalkulacija pri nabavci)
	AzurirajCene(ctx context.Context, id int64, nabavna, prodajna float64) error
	PremestiKategoriju(ctx context.Context, id int64, kategorijaID *int64) error
	Obrisi(ctx context.Context, id int64) error
	// Arhiviraj označava artikal kao arhiviran (skriva ga iz aktivne liste, čuva istoriju)
	Arhiviraj(ctx context.Context, id int64) error
	// Vrati poništava arhiviranje i vraća artikal u aktivnu listu
	Vrati(ctx context.Context, id int64) error
	// SledecaSifra vraća predlog sledeće auto-šifre (npr. KOMP-0042 ili ART-0042)
	SledecaSifra(ctx context.Context, kategorijaID *int64) (string, error)
	// KorigujKolicinu postavlja novu količinu artikla i upisuje korekciju u magacinske_promene
	KorigujKolicinu(ctx context.Context, artikalID int64, novaKolicina int, korisnikID *int64, napomena string) error
	// DobavljaciArtikla vraća ID-jeve dobavljača vezanih za artikal
	DobavljaciArtikla(ctx context.Context, artikalID int64) ([]int64, error)
	// PostaviDobavljaceArtikla zamenjuje skup dobavljača artikla datim ID-jevima
	PostaviDobavljaceArtikla(ctx context.Context, artikalID int64, dobavljaciID []int64) error
	// PoveziDobavljaca dodaje vezu artikal–dobavljač ako ne postoji (auto pri nabavci)
	PoveziDobavljaca(ctx context.Context, artikalID, dobavljacID int64) error
	// OdveziDobavljaca uklanja vezu artikal–dobavljač
	OdveziDobavljaca(ctx context.Context, artikalID, dobavljacID int64) error
	// SveDobavljaceArtikala vraća mapu artikal_id → lista dobavljac_id (za filter u nabavci)
	SveDobavljaceArtikala(ctx context.Context) (map[int64][]int64, error)
}

// KategorijaRepository definiše operacije nad kategorijama
type KategorijaRepository interface {
	Lista(ctx context.Context) ([]model.Kategorija, error)
	DohvatiID(ctx context.Context, id int64) (*model.Kategorija, error)
	Kreiraj(ctx context.Context, k *model.Kategorija) (int64, error)
	Izmeni(ctx context.Context, k *model.Kategorija) error
}

// PdvStopaRepository definiše operacije nad šifarnikom PDV stopa
type PdvStopaRepository interface {
	Lista(ctx context.Context, samoAktivne bool) ([]model.PdvStopa, error)
	DohvatiID(ctx context.Context, id int64) (*model.PdvStopa, error)
	Kreiraj(ctx context.Context, s *model.PdvStopa) (int64, error)
	Izmeni(ctx context.Context, s *model.PdvStopa) error
	PostaviAktivnu(ctx context.Context, id int64, aktivna bool) error
}

// PdvKirRepository definiše operacije nad knjigom izdatih računa (KIR)
type PdvKirRepository interface {
	Lista(ctx context.Context, od, do time.Time) ([]model.PdvKir, error)
	DohvatiID(ctx context.Context, id int64) (*model.PdvKir, error)
	Kreiraj(ctx context.Context, k *model.PdvKir) (int64, error)
	Obrisi(ctx context.Context, id int64) error
	// ObrisiPoIzvoru briše zapise vezane za dati izvor (npr. pri stornu prodaje)
	ObrisiPoIzvoru(ctx context.Context, izvor string, izvorID int64) error
}

// PdvKprRepository definiše operacije nad knjigom primljenih računa (KPR)
type PdvKprRepository interface {
	Lista(ctx context.Context, od, do time.Time) ([]model.PdvKpr, error)
	DohvatiID(ctx context.Context, id int64) (*model.PdvKpr, error)
	Kreiraj(ctx context.Context, k *model.PdvKpr) (int64, error)
	Obrisi(ctx context.Context, id int64) error
	// ObrisiPoIzvoru briše zapise vezane za dati izvor (npr. pri brisanju nabavke)
	ObrisiPoIzvoru(ctx context.Context, izvor string, izvorID int64) error
}

// NivelacijaRepository definiše operacije nad evidencijom promene prodajnih cena
type NivelacijaRepository interface {
	// PromeniCenu transakciono menja prodajnu cenu artikla i upisuje nivelacioni zapis;
	// vraća kreirani zapis (sa starom i novom cenom). Izvor je "rucno".
	PromeniCenu(ctx context.Context, artikalID int64, novaCena float64, razlog string, korisnikID *int64) (*model.Nivelacija, error)
	// Kreiraj upisuje gotov nivelacioni zapis (npr. auto-trag pri izmeni artikla)
	Kreiraj(ctx context.Context, n *model.Nivelacija) (int64, error)
	// Lista vraća nivelacije u periodu (po datumu); nulti datum znači bez granice
	Lista(ctx context.Context, od, do time.Time) ([]model.Nivelacija, error)
	// ListaZaArtikal vraća sve nivelacije jednog artikla (najnovije prvo)
	ListaZaArtikal(ctx context.Context, artikalID int64) ([]model.Nivelacija, error)
}

// ArtikalFilter definiše parametre za filtriranje liste artikala
type ArtikalFilter struct {
	Pretraga     string
	KategorijaID *int64
	Tip          string // 'proizvod' | 'usluga' | 'trosak'; prazno → svi tipovi
	SamoKriticni bool
	Arhivirani   bool // true → vrati samo arhivirane; false (podrazumevano) → samo aktivne
	Limit        int
	Offset       int
}

// UslugaFilter su parametri za listanje usluga
type UslugaFilter struct {
	Pretraga   string
	Arhivirani bool
	Limit      int
	Offset     int
}

// UslugaRepository definiše operacije nad uslugama (zaseban entitet od artikala)
type UslugaRepository interface {
	Lista(ctx context.Context, filter UslugaFilter) ([]model.Usluga, error)
	PrebrojiPoFilteru(ctx context.Context, filter UslugaFilter) (int, error)
	DohvatiID(ctx context.Context, id int64) (*model.Usluga, error)
	Kreiraj(ctx context.Context, u *model.Usluga) (int64, error)
	Izmeni(ctx context.Context, u *model.Usluga) error
	Obrisi(ctx context.Context, id int64) error
	// Kategorije vraća postojeće (distinct) kategorije usluga za predlog pri unosu
	Kategorije(ctx context.Context) ([]string, error)
	// SledecaSifra vraća predlog sledeće auto-šifre usluge (USL-001 … USL-999)
	SledecaSifra(ctx context.Context) (string, error)
}

// TrosakFilter su parametri za listanje troškova
type TrosakFilter struct {
	Pretraga   string
	Arhivirani bool
	Limit      int
	Offset     int
}

// TrosakRepository definiše operacije nad šifrarnikom vrsta troškova
type TrosakRepository interface {
	Lista(ctx context.Context, filter TrosakFilter) ([]model.Trosak, error)
	DohvatiID(ctx context.Context, id int64) (*model.Trosak, error)
	Kreiraj(ctx context.Context, t *model.Trosak) (int64, error)
	Izmeni(ctx context.Context, t *model.Trosak) error
	Obrisi(ctx context.Context, id int64) error
	// SledecaSifra vraća predlog sledeće auto-šifre troška (TRO-001 … TRO-999)
	SledecaSifra(ctx context.Context) (string, error)
}

// NabavkaRepository definiše operacije nad nabavkama
type NabavkaRepository interface {
	Lista(ctx context.Context) ([]model.NabavkaSaDetaljem, error)
	DohvatiID(ctx context.Context, id int64) (*model.Nabavka, error)
	DohvatiStavke(ctx context.Context, nabavkaID int64) ([]model.StavkaSaArtiklom, error)
	DohvatiTroskove(ctx context.Context, nabavkaID int64) ([]model.NabavkaTrosak, error)
	Kreiraj(ctx context.Context, n *model.Nabavka, stavke []model.StavkaNabavke, troskovi []model.NabavkaTrosak, korisnikID *int64) (int64, error)
	Obrisi(ctx context.Context, id int64, korisnikID *int64) error
}

// DobavljacRepository definiše operacije nad dobavljačima
type DobavljacRepository interface {
	Lista(ctx context.Context, pretraga string) ([]model.Dobavljac, error)
	DohvatiID(ctx context.Context, id int64) (*model.Dobavljac, error)
	Kreiraj(ctx context.Context, d *model.Dobavljac) (int64, error)
	Izmeni(ctx context.Context, d *model.Dobavljac) error
	Obrisi(ctx context.Context, id int64) error
}

// KlijentFilter definiše parametre za filtriranje liste klijenata
type KlijentFilter struct {
	Pretraga string
	Tip      string // "fizicko", "pravno" ili "" za sve
	Limit    int
	Offset   int
}

// KlijentRepository definiše operacije nad klijentima
type KlijentRepository interface {
	Lista(ctx context.Context, pretraga string) ([]model.Klijent, error)
	ListaFilter(ctx context.Context, filter KlijentFilter) ([]model.Klijent, error)
	PrebrojiPoFilteru(ctx context.Context, filter KlijentFilter) (int, error)
	DohvatiID(ctx context.Context, id int64) (*model.Klijent, error)
	Pronadji(ctx context.Context, tip, ime, prezime, nazivFirme, jmbg, telefon, email, mesto string) (*model.Klijent, error)
	Kreiraj(ctx context.Context, k *model.Klijent) (int64, error)
	Izmeni(ctx context.Context, k *model.Klijent) error
	Obrisi(ctx context.Context, id int64) error
}

// ServisRepository definiše operacije nad servisnim nalozima
type ServisRepository interface {
	Lista(ctx context.Context, pretraga, status string) ([]model.ServisniNalogSaKlijentom, error)
	DohvatiID(ctx context.Context, id int64) (*model.ServisniNalog, error)
	DohvatiJavniToken(ctx context.Context, token string) (*model.ServisniNalog, error)
	Kreiraj(ctx context.Context, n *model.ServisniNalog) (int64, error)
	Izmeni(ctx context.Context, n *model.ServisniNalog) error
	AzurirajStatus(ctx context.Context, id int64, status string) error
	AzurirajGaranciju(ctx context.Context, id int64, garancijaDo *time.Time) error
	AzurirajGarancijaDana(ctx context.Context, id int64, dana *int) error
	AzurirajPredvidjenDatum(ctx context.Context, id int64, predvidjenDatum *time.Time) error
	AzurirajTehnicar(ctx context.Context, id int64, tehnicarID *int64) error
	AzurirajNapomenuKlijentu(ctx context.Context, id int64, tekst string) error
	AzurirajNalazDijagnostike(ctx context.Context, id int64, tekst string) error
	AzurirajUradjeno(ctx context.Context, id int64, tekst string) error
	AzurirajCenaDijagnostike(ctx context.Context, id int64, cena float64) error
	AzurirajCenuKonacnu(ctx context.Context, id int64, cena float64) error
	OdbijPopravku(ctx context.Context, id int64, cena float64) error
	AzurirajKomentarKlijenta(ctx context.Context, id int64, tekst string) error
	SacuvajOdlukuKlijenta(ctx context.Context, id int64, odluka string, odgovor string) error
	ObrisiOdlukuKlijenta(ctx context.Context, id int64) error
	Obrisi(ctx context.Context, id int64, korisnikID *int64) error
	SledeciBroj(ctx context.Context) (string, error)
	SacuvajNaplatu(ctx context.Context, id int64, nacinPlacanja string, naplaceno float64) error
}

// ProdajaRepository definiše operacije nad prodajnim nalozima
type ProdajaRepository interface {
	Lista(ctx context.Context, pretraga string) ([]model.ProdajniNalogSaDetaljem, error)
	DohvatiID(ctx context.Context, id int64) (*model.ProdajniNalog, error)
	DohvatiStavke(ctx context.Context, nalogID int64) ([]model.StavkaProdajeSaArtiklom, error)
	Kreiraj(ctx context.Context, n *model.ProdajniNalog, stavke []model.StavkaProdaje, korisnikID *int64) (int64, error)
	Storno(ctx context.Context, id int64, razlog string, korisnikID *int64) error
	Obrisi(ctx context.Context, id int64, korisnikID *int64) error
	SledeciBroj(ctx context.Context) (string, error)
}

// ServisniDeloviRepository definiše operacije nad ugrađenim delovima u servisu
type ServisniDeloviRepository interface {
	DohvatiZaNalog(ctx context.Context, nalogID int64) ([]model.ServisniDeoSaArtiklom, error)
	// UgradiIliPotrazuj atomično (jedna transakcija) čita stanje magacina, ugrađuje
	// ono što fizički ima (skida sa lagera, lager NE ide u minus), a višak beleži u
	// potraživane delove. Vraća koliko je ugrađeno i koliko nedostaje.
	UgradiIliPotrazuj(ctx context.Context, nalogID, artikalID int64, kolicina int, cenaKomada float64, korisnikID *int64, predlozeno bool) (ugradjeno, nedostaje int, err error)
	DohvatiArtikalID(ctx context.Context, deoID int64) (int64, error)
	Obrisi(ctx context.Context, id int64, korisnikID *int64) error
	// PrihvatiPredlozene prebacuje predložene delove (i potraživane) naloga u ugrađene
	PrihvatiPredlozene(ctx context.Context, nalogID int64) error
	// ObrisiPredlozene briše sve predložene delove sa naloga (klijent odbio predlog)
	ObrisiPredlozene(ctx context.Context, nalogID int64) error
	// PrihvatiOdabranePoArtiklu selektivno prihvata predlozene delove po artikal_id;
	// prihvaćeni idu kroz normalnu ugradnju (skidaju sa lagera), ostali se odbacuju.
	PrihvatiOdabranePoArtiklu(ctx context.Context, nalogID int64, artikalIDs []int64) error
}

// ServisniPotrazivaniDeloviRepository definiše operacije nad delovima koji nedostaju
type ServisniPotrazivaniDeloviRepository interface {
	DohvatiZaNalog(ctx context.Context, nalogID int64) ([]model.ServisniPotrazivaniDeo, error)
	Obrisi(ctx context.Context, id int64) error
	ObrisiZaArtikal(ctx context.Context, nalogID, artikalID int64) error
	ObrisiPredlozeneZaArtikal(ctx context.Context, nalogID, artikalID int64) error
	// ProveriIPocistiZaArtikal proverava potraživane redove za dati artikal nakon
	// povećanja stanja i briše/smanjuje redove koji se mogu pokriti (FIFO).
	// Vraća ID-eve naloga čiji je poslednji potraživani red obrisan.
	ProveriIPocistiZaArtikal(ctx context.Context, artikalID int64) ([]int64, error)
}

// ServisniRadoviRepository definiše operacije nad radovima (uslugama) na nalogu
type ServisniRadoviRepository interface {
	DohvatiZaNalog(ctx context.Context, nalogID int64) ([]model.ServisniRad, error)
	Dodaj(ctx context.Context, nalogID, uslugaID int64, naziv string, kolicina, cenaKomada float64, predlozeno bool) (int64, error)
	Obrisi(ctx context.Context, id int64) error
	// PrihvatiPredlozene prebacuje predložene radove naloga u redovne
	PrihvatiPredlozene(ctx context.Context, nalogID int64) error
	// ObrisiPredlozene briše sve predložene radove sa naloga (klijent odbio predlog)
	ObrisiPredlozene(ctx context.Context, nalogID int64) error
	// PrihvatiOdabrane selektivno prihvata predlozene radove po ID-u;
	// prihvaćeni dobijaju predlozeno=0, ostali predlozeni se brišu.
	PrihvatiOdabrane(ctx context.Context, nalogID int64, ids []int64) error
}

// MagacinskePromeneRepository definiše operacije nad revizijskim tragom magacina
type MagacinskePromeneRepository interface {
	Lista(ctx context.Context, artikalID *int64, limit int) ([]model.MagacinskaPromenaSaDetaljem, error)
}

// KorisniciRepository definiše operacije nad korisnicima
type KorisniciRepository interface {
	Kreiraj(ctx context.Context, korisnickoIme, lozinkaHash, uloga string) (*model.Korisnik, error)
	DohvatiPoImenu(ctx context.Context, korisnickoIme string) (*model.Korisnik, error)
	DohvatiPoID(ctx context.Context, id int64) (*model.Korisnik, error)
	Lista(ctx context.Context) ([]model.Korisnik, error)
	AzurirajUlogu(ctx context.Context, id int64, uloga string) error
	AzurirajAktivan(ctx context.Context, id int64, aktivan bool) error
	AzurirajImePrezime(ctx context.Context, id int64, ime, prezime string) error
	PromeniLozinku(ctx context.Context, id int64, hash string) error
	SacuvajTotpTajnu(ctx context.Context, id int64, tajna string) error
	SacuvajLokalnuTemu(ctx context.Context, id int64, lokalnaTema string, koristi bool) error
	SacuvajLokalnuPozadinu(ctx context.Context, id int64, pozadina, opacity, blur, blurPozadine, glassOpacity string) error
	SacuvajLokalnuAnimaciju(ctx context.Context, id int64, animacija string) error
	SacuvajLokalniHover(ctx context.Context, id int64, hover string) error
	SacuvajLokalnuBrzinuAnimacije(ctx context.Context, id int64, brzina string) error
	SacuvajAvatar(ctx context.Context, id int64, putanja string) error
	PostojiIjedan(ctx context.Context) (bool, error)
	Obrisi(ctx context.Context, id int64) error
}

// SesijeRepository definiše operacije nad sesijama
type SesijeRepository interface {
	Kreiraj(ctx context.Context, korisnikID int64, token string, istice time.Time, totpPotvrdjeno bool) error
	DohvatiPoTokenu(ctx context.Context, token string) (*model.Sesija, error)
	PotvrdiTotp(ctx context.Context, token string, novoIstice time.Time) error
	Obrisi(ctx context.Context, token string) error
	ObrisiIstekle(ctx context.Context) error
}

// PodsetnikFilter definiše parametre za filtriranje liste podsetnika
type PodsetnikFilter struct {
	SamoAktivni bool   // true = samo nezavršeni; false = svi
	KorisnikID  *int64 // ako nije nil — samo podsetnici tog korisnika
}

// PokusajiPrijaveRepository definiše operacije nad evidencijom pokušaja prijave
type PokusajiPrijaveRepository interface {
	Zabeleži(ctx context.Context, ip, korisnickoIme string, uspeh bool) error
	BrojNeuspeha(ctx context.Context, ip string, od time.Time) (int, error)
	VremePoslednjeg(ctx context.Context, ip string, od time.Time) (time.Time, bool, error)
	ObrisiStare(ctx context.Context, pre time.Time) error
}

// LoginIstorijsaRepository definiše operacije nad evidencijom prijava
type LoginIstorijsaRepository interface {
	Zabeleži(ctx context.Context, korisnikID *int64, ip, userAgent, razlog string, uspeh bool) error
	ListaZaKorisnika(ctx context.Context, korisnikID int64, limit int) ([]*model.LoginPokusaj, error)
}

// DozvoleRepository definiše operacije nad dozvolama po ulogama
type DozvoleRepository interface {
	ImaDozvolu(ctx context.Context, uloga, akcija string) bool
	SveDozvole(ctx context.Context, uloga string) map[string]bool
	Sacuvaj(ctx context.Context, uloga, akcija string, dozvoljeno bool) error
	Reset(ctx context.Context) error
}

// RezervniKodoviRepository definiše operacije nad rezervnim (jednokratnim) 2FA kodovima.
// Kodovi se čuvaju kao bcrypt heš; Iskoristi prima čist kod i poredi ga sa hešovima.
type RezervniKodoviRepository interface {
	Zameni(ctx context.Context, korisnikID int64, hashevi []string) error
	Iskoristi(ctx context.Context, korisnikID int64, kod string) (bool, error)
	BrojPreostalih(ctx context.Context, korisnikID int64) (int, error)
	Obrisi(ctx context.Context, korisnikID int64) error
}

// IzvestajRepository definiše read-only upite za dashboard i stranicu izveštaja.
// Vraća sirove podatke; prezentaciju (datumi, boje, rang) radi handler.
type IzvestajRepository interface {
	// dashboard — brojači
	BrojArtikala(ctx context.Context) (int, error)
	BrojAktivnihServisa(ctx context.Context) (int, error)
	PrihodTekuciMesec(ctx context.Context) (float64, error)
	BrojKriticnihZaliha(ctx context.Context) (int, error)
	// dashboard — liste
	PoslednjiServisi(ctx context.Context, limit int) ([]model.ServisRedDashboard, error)
	KriticneZalihe(ctx context.Context, limit int) ([]model.ZalihaRed, error)
	PoslednjeProdaje(ctx context.Context, limit int) ([]model.ProdajaRedDashboard, error)
	// izveštaji
	MesecniPrihodProdaja(ctx context.Context) ([]model.MesecniIznos, error)
	MesecniPrihodServis(ctx context.Context) ([]model.MesecniIznos, error)
	StariOtvoreniNalozi(ctx context.Context) ([]model.StariNalogRed, error)
	TopArtikli(ctx context.Context, limit int) ([]model.TopArtikalRed, error)
	TopKlijenti(ctx context.Context, limit int) ([]model.TopKlijentRed, error)
	// magacinski izveštaji
	PrometniList(ctx context.Context, od, do time.Time) ([]model.PrometniRed, error)
	StanjeZaliha(ctx context.Context) ([]model.StanjeZalihaRed, error)
}

// PodsetnikRepository definiše operacije nad podsetnicima
type PodsetnikRepository interface {
	Lista(ctx context.Context, filter PodsetnikFilter) ([]model.Podsetnik, error)
	DohvatiID(ctx context.Context, id int64) (*model.Podsetnik, error)
	Kreiraj(ctx context.Context, p *model.Podsetnik) (int64, error)
	Izmeni(ctx context.Context, p *model.Podsetnik) error
	OznaciZavrsenim(ctx context.Context, id int64, zavrseno bool) error
	Obrisi(ctx context.Context, id int64) error
	BrojAktivnih(ctx context.Context, filter PodsetnikFilter) (int, error)
}

// FiskalRepository definiše operacije nad fiskalnim računima (Teron L-PFR).
// Svaki prodajni nalog ima najviše jedan fiskalni račun (UNIQUE prodaja_id).
type FiskalRepository interface {
	Kreiraj(ctx context.Context, fr *model.FiskalniRacun) (int64, error)
	DohvatiPoProdaji(ctx context.Context, prodajaID int64) (*model.FiskalniRacun, error)
	DohvatiPoServisu(ctx context.Context, servisID int64) (*model.FiskalniRacun, error)
}
