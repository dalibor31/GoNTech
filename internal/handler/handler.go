package handler

import (
	"database/sql"
	"html/template"
	"io/fs"
	"net/http"

	"ntech/internal/db"
	"ntech/internal/db/sqlite"
	"ntech/internal/middleware"
	"ntech/internal/model"
)

// Handler drži zavisnosti koje su potrebne svim handlerima
type Handler struct {
	DB          *sql.DB
	PutanjaBaze string
	Artikli         db.ArtikalRepository
	KategorijeRepo  db.KategorijaRepository
	DobavljaciRepo  db.DobavljacRepository
	NabavkeRepo     db.NabavkaRepository
	KlijentiRepo    db.KlijentRepository
	ServisRepo          db.ServisRepository
	ServisniDeloviRepo  db.ServisniDeloviRepository
	MagacinskePromeneRepo db.MagacinskePromeneRepository
	ProdajaRepo     db.ProdajaRepository
	KorisniciRepo   db.KorisniciRepository
	SesijeRepo      db.SesijeRepository
	PodsetnikRepo db.PodsetnikRepository
	PokusajiRepo       db.PokusajiPrijaveRepository
	LoginIstorijsaRepo db.LoginIstorijsaRepository
	DozvoleRepo     db.DozvoleRepository
	Verzija     string
	AssetV      string // verzija statičkih fajlova za cache-busting (postavlja se pri pokretanju)
	Templates   map[string]*template.Template
	TemplatesFS fs.FS
	totpKljuc   []byte // ključ za šifrovanje TOTP tajni (prosleđuje se KorisniciRepo)
}

// Novi kreira novi Handler sa datom bazom.
// totpKljuc je 32-bajtni ključ za šifrovanje TOTP tajni u mirovanju.
func Novi(baza *sql.DB, totpKljuc []byte) *Handler {
	return &Handler{
		DB:             baza,
		totpKljuc:      totpKljuc,
		Artikli:        sqlite.NoviArtikalRepo(baza),
		KategorijeRepo: sqlite.NovaKategorijaRepo(baza),
		DobavljaciRepo: sqlite.NoviDobavljacRepo(baza),
		NabavkeRepo:    sqlite.NoviNabavkaRepo(baza),
		KlijentiRepo:   sqlite.NoviKlijentRepo(baza),
		ServisRepo:            sqlite.NoviServisRepo(baza),
		ServisniDeloviRepo:    sqlite.NoviServisniDeloviRepo(baza),
		MagacinskePromeneRepo: sqlite.NoviMagacinskePromeneRepo(baza),
		ProdajaRepo:           sqlite.NoviProdajaRepo(baza),
		KorisniciRepo:   sqlite.NoviKorisniciRepo(baza, totpKljuc),
		SesijeRepo:      sqlite.NoviSesijeRepo(baza),
		PodsetnikRepo: sqlite.NoviPodsetnikRepo(baza),
		PokusajiRepo:       sqlite.NoviPokusajiPrijaveRepo(baza),
		LoginIstorijsaRepo: sqlite.NoviLoginIstorijsaRepo(baza),
		DozvoleRepo:     sqlite.NoviDozvoleRepo(baza, middleware.ImaDozvolu, middleware.SveAkcije()),
	}
}

// reinicijalizujRepozitorijume zamenjuje sve repozitorijume posle obnove baze
func (h *Handler) reinicijalizujRepozitorijume(novaDB *sql.DB) {
	h.DB = novaDB
	h.Artikli = sqlite.NoviArtikalRepo(novaDB)
	h.KategorijeRepo = sqlite.NovaKategorijaRepo(novaDB)
	h.DobavljaciRepo = sqlite.NoviDobavljacRepo(novaDB)
	h.NabavkeRepo = sqlite.NoviNabavkaRepo(novaDB)
	h.KlijentiRepo = sqlite.NoviKlijentRepo(novaDB)
	h.ServisRepo = sqlite.NoviServisRepo(novaDB)
	h.ServisniDeloviRepo = sqlite.NoviServisniDeloviRepo(novaDB)
	h.MagacinskePromeneRepo = sqlite.NoviMagacinskePromeneRepo(novaDB)
	h.ProdajaRepo = sqlite.NoviProdajaRepo(novaDB)
	h.KorisniciRepo = sqlite.NoviKorisniciRepo(novaDB, h.totpKljuc)
	h.SesijeRepo = sqlite.NoviSesijeRepo(novaDB)
	h.PodsetnikRepo = sqlite.NoviPodsetnikRepo(novaDB)
	h.PokusajiRepo = sqlite.NoviPokusajiPrijaveRepo(novaDB)
	h.LoginIstorijsaRepo = sqlite.NoviLoginIstorijsaRepo(novaDB)
	h.DozvoleRepo = sqlite.NoviDozvoleRepo(novaDB, middleware.ImaDozvolu, middleware.SveAkcije())
}

// zahtevajDozvolu vraća prijavljenog korisnika ako njegova uloga sme da izvrši akciju.
// U suprotnom šalje 403 sa srpskom porukom i vraća ok=false (handler tada return-uje).
func (h *Handler) zahtevajDozvolu(w http.ResponseWriter, r *http.Request, akcija string) (*model.Korisnik, bool) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if k == nil || !h.DozvoleRepo.ImaDozvolu(r.Context(), k.Uloga, akcija) {
		http.Error(w, "Nemate dozvolu za ovu akciju.", http.StatusForbidden)
		return nil, false
	}
	return k, true
}

// popuniPodaciStranice popunjava zajednička polja stranice uključujući prijavljenog korisnika
func (h *Handler) popuniPodaciStranice(r *http.Request, podesavanja map[string]string) model.PodaciStranice {
	// podrazumevana tema je tamna; korisnik može imati svoju lokalnu temu
	tema := "tamna"

	ps := model.PodaciStranice{
		Tema:        tema,
		NazivFirme:  podesavanja["naziv_firme"],
		Podnazlov:   podesavanja["podnazlov"],
		LogoTip:     podesavanja["logo_tip"],
		LogoPutanja: podesavanja["logo_putanja"],
		Korisnik:    "Admin",
	}
	var korisnik *model.Korisnik
	if k := middleware.KorisnikIzKonteksta(r.Context()); k != nil {
		korisnik = k
		ps.Korisnik = k.KorisnickoIme
		ps.KorisnikIme = k.KorisnickoIme
		ps.KorisnikUloga = k.Uloga
		ps.Dozvole = h.DozvoleRepo.SveDozvole(r.Context(), k.Uloga)
		// lokalna tema korisnika — primenjuje se uvek kada je postavljena, bez obzira na KoristiLokalnuTemu
		if k.LokalnaTema != "" {
			ps.Tema = k.LokalnaTema
		}
	}
	ps.CsrfToken = middleware.CsrfToken(r.Context())
	ps.AssetV = h.AssetV
	ps.Flash = middleware.GetFlash(r, h.DB)

	// logika pozadine:
	// - lična pozadina → uvek se prikazuje i forsira tamnu temu, bez obzira na KoristiLokalnuTemu
	// - globalna pozadina → prikazuje se svima koji nemaju ličnu
	// KoristiLokalnuTemu i LokalnaTema važe samo kada nema lične pozadine
	if korisnik != nil && korisnik.LokalnaPozadina != "" {
		ps.AppPozadina = korisnik.LokalnaPozadina
		ps.Tema = "tamna"
		ps.AppPozadinaOpacity = korisnik.LokalnaPozadinaOpacity
		if ps.AppPozadinaOpacity == "" {
			ps.AppPozadinaOpacity = "50"
		}
		ps.AppPozadinaBlur = korisnik.LokalnaPozadinaBlur
		if ps.AppPozadinaBlur == "" {
			ps.AppPozadinaBlur = "12"
		}
		ps.AppPozadinaBlurPozadine = korisnik.LokalnaPozadinaBlurPozadine
		if ps.AppPozadinaBlurPozadine == "" {
			ps.AppPozadinaBlurPozadine = "0"
		}
		ps.AppPozadinaGlassOpacity = korisnik.LokalnaPozadinaGlassOpacity
		if ps.AppPozadinaGlassOpacity == "" {
			ps.AppPozadinaGlassOpacity = "10"
		}
	}

	return ps
}
