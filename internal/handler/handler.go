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
	ServisRepo      db.ServisRepository
	ProdajaRepo     db.ProdajaRepository
	KorisniciRepo   db.KorisniciRepository
	SesijeRepo      db.SesijeRepository
	PodsetniciFRepo db.PodsetnikRepository
	PokusajiRepo       db.PokusajiPrijaveRepository
	LoginIstorijsaRepo db.LoginIstorijsaRepository
	DozvoleRepo     db.DozvoleRepository
	Verzija     string
	Templates   map[string]*template.Template
	TemplatesFS fs.FS
}

// Novi kreira novi Handler sa datom bazom
func Novi(baza *sql.DB) *Handler {
	return &Handler{
		DB:             baza,
		Artikli:        sqlite.NoviArtikalRepo(baza),
		KategorijeRepo: sqlite.NovaKategorijaRepo(baza),
		DobavljaciRepo: sqlite.NoviDobavljacRepo(baza),
		NabavkeRepo:    sqlite.NoviNabavkaRepo(baza),
		KlijentiRepo:   sqlite.NoviKlijentRepo(baza),
		ServisRepo:     sqlite.NoviServisRepo(baza),
		ProdajaRepo:    sqlite.NoviProdajaRepo(baza),
		KorisniciRepo:   sqlite.NoviKorisniciRepo(baza),
		SesijeRepo:      sqlite.NoviSesijeRepo(baza),
		PodsetniciFRepo: sqlite.NoviPodsetnikRepo(baza),
		PokusajiRepo:       sqlite.NoviPokusajiPrijaveRepo(baza),
		LoginIstorijsaRepo: sqlite.NoviLoginIstorijsaRepo(baza),
		DozvoleRepo:     sqlite.NoviDozvoleRepo(baza, middleware.ImaDozvolu, middleware.SveAkcije()),
	}
}

// reinicijalzijRepozitorijume zamenjuje sve repozitorijume posle obnove baze
func (h *Handler) reinicijalzijRepozitorijume(novaDB *sql.DB) {
	h.DB = novaDB
	h.Artikli = sqlite.NoviArtikalRepo(novaDB)
	h.KategorijeRepo = sqlite.NovaKategorijaRepo(novaDB)
	h.DobavljaciRepo = sqlite.NoviDobavljacRepo(novaDB)
	h.NabavkeRepo = sqlite.NoviNabavkaRepo(novaDB)
	h.KlijentiRepo = sqlite.NoviKlijentRepo(novaDB)
	h.ServisRepo = sqlite.NoviServisRepo(novaDB)
	h.ProdajaRepo = sqlite.NoviProdajaRepo(novaDB)
	h.KorisniciRepo = sqlite.NoviKorisniciRepo(novaDB)
	h.SesijeRepo = sqlite.NoviSesijeRepo(novaDB)
	h.PodsetniciFRepo = sqlite.NoviPodsetnikRepo(novaDB)
	h.PokusajiRepo = sqlite.NoviPokusajiPrijaveRepo(novaDB)
	h.LoginIstorijsaRepo = sqlite.NoviLoginIstorijsaRepo(novaDB)
	h.DozvoleRepo = sqlite.NoviDozvoleRepo(novaDB, middleware.ImaDozvolu, middleware.SveAkcije())
}

// popuniPodaciStranice popunjava zajednička polja stranice uključujući prijavljenog korisnika
func (h *Handler) popuniPodaciStranice(r *http.Request, podesavanja map[string]string) model.PodaciStranice {
	// redosled prioriteta teme: pozadinska slika → lokalna → globalna → fallback
	globalnaTema := podesavanja["globalna_tema"]
	if globalnaTema == "" {
		globalnaTema = podesavanja["tema"]
	}
	if globalnaTema == "" {
		globalnaTema = "tamna"
	}
	tema := globalnaTema

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
		// lokalna tema korisnika
		if k.KoristiLokalnuTemu && k.LokalnaTema != "" {
			ps.Tema = k.LokalnaTema
		}
	}
	ps.CsrfToken = middleware.CsrfToken(r.Context())
	ps.Flash = middleware.GetFlash(r, h.DB)

	// logika pozadine:
	// - lična pozadina (samo kada je lokalni režim aktivan) → zamenjuje globalnu
	// - globalna pozadina → prikazuje se svima koji nemaju ličnu
	// KoristiLokalnuTemu utiče na izbor tamne/svetle teme, ne na vidljivost pozadine
	if korisnik != nil && korisnik.KoristiLokalnuTemu && korisnik.LokalnaPozadina != "" {
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
	} else {
		ps.AppPozadina = podesavanja["app_pozadina"]
		if ps.AppPozadina != "" {
			// globalna pozadina forsira tamnu temu, osim ako korisnik ima aktivnu lokalnu temu
			if korisnik == nil || !korisnik.KoristiLokalnuTemu {
				ps.Tema = "tamna"
			}
			ps.AppPozadinaOpacity = podesavanja["app_pozadina_opacity"]
			if ps.AppPozadinaOpacity == "" {
				ps.AppPozadinaOpacity = "50"
			}
			ps.AppPozadinaBlur = podesavanja["app_pozadina_blur"]
			if ps.AppPozadinaBlur == "" {
				ps.AppPozadinaBlur = "12"
			}
			ps.AppPozadinaBlurPozadine = podesavanja["app_pozadina_blur_pozadine"]
			if ps.AppPozadinaBlurPozadine == "" {
				ps.AppPozadinaBlurPozadine = "0"
			}
		} else {
			ps.AppPozadinaOpacity = podesavanja["app_pozadina_opacity"]
			if ps.AppPozadinaOpacity == "" {
				ps.AppPozadinaOpacity = "50"
			}
			ps.AppPozadinaBlur = podesavanja["app_pozadina_blur"]
			if ps.AppPozadinaBlur == "" {
				ps.AppPozadinaBlur = "12"
			}
			ps.AppPozadinaBlurPozadine = podesavanja["app_pozadina_blur_pozadine"]
			if ps.AppPozadinaBlurPozadine == "" {
				ps.AppPozadinaBlurPozadine = "0"
			}
		}
	}

	return ps
}
