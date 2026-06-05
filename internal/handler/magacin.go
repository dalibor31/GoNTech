package handler

import (
	"net/http"
	"strconv"

	"ntech/internal/db"
	"ntech/internal/db/sqlite"
	"ntech/internal/middleware"
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

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "magacin"
	ps.NaslovStranice = "Magacin"
	podaci := PodaciMagacina{
		PodaciStranice:  ps,
		Artikli:         artikli,
		Kategorije:      kategorije,
		Filter:          filter,
		KategorijaIDStr: katIDStr,
		Sacuvano:        r.URL.Query().Get("sacuvano") == "1",
		Obrisan:         r.URL.Query().Get("obrisan") == "1",
	}

	h.renderujTemplate(w, "magacin", podaci)
}

// ObrisiArtikal briše artikal po ID-u
func (h *Handler) ObrisiArtikal(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if !h.DozvoleRepo.ImaDozvolu(r.Context(), k.Uloga, "artikal.obrisi") {
		http.Error(w, "Nemate dozvolu za ovu akciju.", http.StatusForbidden)
		return
	}
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
