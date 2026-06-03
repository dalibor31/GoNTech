package handler

import (
	"database/sql"
	"html/template"

	"ntech/internal/db"
	"ntech/internal/db/sqlite"
	"ntech/internal/middleware"
	"ntech/internal/model"
	"net/http"
)

// Handler drži zavisnosti koje su potrebne svim handlerima
type Handler struct {
	DB              *sql.DB
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
	Verzija            string
	Templates       map[string]*template.Template
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
	}
}

// popuniPodaciStranice popunjava zajednička polja stranice uključujući prijavljenog korisnika
func (h *Handler) popuniPodaciStranice(r *http.Request, podesavanja map[string]string) model.PodaciStranice {
	ps := model.PodaciStranice{
		Tema:        podesavanja["tema"],
		NazivFirme:  podesavanja["naziv_firme"],
		Podnazlov:   podesavanja["podnazlov"],
		LogoTip:     podesavanja["logo_tip"],
		LogoPutanja: podesavanja["logo_putanja"],
		Korisnik:    "Admin",
	}
	if k := middleware.KorisnikIzKonteksta(r.Context()); k != nil {
		ps.Korisnik = k.KorisnickoIme
		ps.KorisnikIme = k.KorisnickoIme
		ps.KorisnikUloga = k.Uloga
	}
	ps.CsrfToken = middleware.CsrfToken(r.Context())
	return ps
}
