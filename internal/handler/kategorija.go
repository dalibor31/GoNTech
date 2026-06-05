package handler

import (
	"net/http"
	"strconv"

	"ntech/internal/db/sqlite"
	"ntech/internal/middleware"
	"ntech/internal/model"

	"github.com/go-chi/chi/v5"
)

// PodaciKategorija su podaci za stranicu kategorija
type PodaciKategorija struct {
	model.PodaciStranice
	Kategorije []model.Kategorija
	Sacuvano   bool
	Obrisana   bool
}

// Kategorije renderuje listu kategorija
func (h *Handler) Kategorije(w http.ResponseWriter, r *http.Request) {
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	kategorije, err := h.KategorijeRepo.Lista(r.Context())
	if err != nil {
		http.Error(w, "Greška pri učitavanju kategorija", http.StatusInternalServerError)
		return
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "magacin"
	ps.NaslovStranice = "Kategorije"
	podaci := PodaciKategorija{
		PodaciStranice: ps,
		Kategorije:     kategorije,
		Sacuvano:       r.URL.Query().Get("sacuvano") == "1",
		Obrisana:       r.URL.Query().Get("obrisana") == "1",
	}

	h.renderujTemplate(w, "kategorije", podaci)
}

// DodajKategoriju prima POST i čuva novu kategoriju
func (h *Handler) DodajKategoriju(w http.ResponseWriter, r *http.Request) {
	kor := middleware.KorisnikIzKonteksta(r.Context())
	if kor == nil || !h.DozvoleRepo.ImaDozvolu(r.Context(), kor.Uloga, "kategorija.dodaj") {
		http.Error(w, "Nemate dozvolu za ovu akciju.", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	naziv := r.FormValue("naziv")
	if naziv == "" {
		http.Redirect(w, r, "/magacin/kategorije", http.StatusSeeOther)
		return
	}

	k := &model.Kategorija{
		Naziv: naziv,
		Opis:  r.FormValue("opis"),
	}

	if _, err := h.KategorijeRepo.Kreiraj(r.Context(), k); err != nil {
		http.Error(w, "Greška pri čuvanju kategorije", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/magacin/kategorije?sacuvano=1", http.StatusSeeOther)
}

// ObrisiKategoriju briše kategoriju po ID-u
func (h *Handler) ObrisiKategoriju(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if !h.DozvoleRepo.ImaDozvolu(r.Context(), k.Uloga, "kategorija.obrisi") {
		http.Error(w, "Nemate dozvolu za ovu akciju.", http.StatusForbidden)
		return
	}
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Neispravan ID kategorije", http.StatusBadRequest)
		return
	}

	if _, err := h.DB.ExecContext(r.Context(), "DELETE FROM kategorije WHERE id = ?", id); err != nil {
		http.Error(w, "Greška pri brisanju kategorije", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/magacin/kategorije?obrisana=1", http.StatusSeeOther)
}
