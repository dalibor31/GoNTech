package handler

import (
	"html/template"
	"log"
	"net/http"
	"strconv"

	"ntech/internal/db"
	"ntech/internal/db/sqlite"
	"ntech/internal/model"

	"github.com/go-chi/chi/v5"
)

// PodaciMagacina su podaci za stranicu magacina
type PodaciMagacina struct {
	model.PodaciStranice
	Artikli         []model.ArtikalSaKategorijom
	Kategorije      []model.Kategorija
	Filter          db.ArtikalFilter
	KategorijaIDStr string
	Sacuvano        bool
	Obrisan         bool
}

// Magacin renderuje listu artikala
func (h *Handler) Magacin(w http.ResponseWriter, r *http.Request) {
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	filter := db.ArtikalFilter{
		Pretraga:     r.URL.Query().Get("pretraga"),
		SamoKriticni: r.URL.Query().Get("kriticni") == "1",
	}

	katIDStr := ""
	if katID := r.URL.Query().Get("kategorija"); katID != "" {
		id, err := strconv.ParseInt(katID, 10, 64)
		if err == nil {
			filter.KategorijaID = &id
			katIDStr = katID
		}
	}

	artikli, err := h.Artikli.Lista(r.Context(), filter)
	if err != nil {
		http.Error(w, "Greška pri učitavanju artikala", http.StatusInternalServerError)
		return
	}

	kategorije, err := h.KategorijeRepo.Lista(r.Context())
	if err != nil {
		http.Error(w, "Greška pri učitavanju kategorija", http.StatusInternalServerError)
		return
	}

	podaci := PodaciMagacina{
		PodaciStranice: model.PodaciStranice{
			Stranica:       "magacin",
			NaslovStranice: "Magacin",
			Tema:           podesavanja["tema"],
			NazivFirme:     podesavanja["naziv_firme"],
			Podnazlov:      podesavanja["podnazlov"],
			LogoTip:        podesavanja["logo_tip"],
			LogoPutanja:    podesavanja["logo_putanja"],
			Korisnik:       "Admin",
		},
		Artikli:         artikli,
		Kategorije:      kategorije,
		Filter:          filter,
		KategorijaIDStr: katIDStr,
		Sacuvano:        r.URL.Query().Get("sacuvano") == "1",
		Obrisan:         r.URL.Query().Get("obrisan") == "1",
	}

	tmpl, err := template.ParseFiles(
		"web/templates/teme/podrazumevana/base.html",
		"web/templates/komponente/sidebar.html",
		"web/templates/komponente/topbar.html",
		"web/templates/stranice/magacin.html",
	)
	if err != nil {
		log.Printf("greška pri učitavanju šablona: %v", err)
		http.Error(w, "Greška pri učitavanju stranice", http.StatusInternalServerError)
		return
	}

	if err := tmpl.ExecuteTemplate(w, "base", podaci); err != nil {
		log.Printf("greška pri renderovanju: %v", err)
		http.Error(w, "Greška pri prikazu stranice", http.StatusInternalServerError)
		return
	}
}

// ObrisiArtikal briše artikal po ID-u
func (h *Handler) ObrisiArtikal(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Neispravan ID artikla", http.StatusBadRequest)
		return
	}

	if err := h.Artikli.Obrisi(r.Context(), id); err != nil {
		http.Error(w, "Greška pri brisanju artikla", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/magacin?obrisan=1", http.StatusSeeOther)
}
