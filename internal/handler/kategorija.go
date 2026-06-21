package handler

import (
	"net/http"
	"strconv"
	"strings"

	"ntech/internal/db/sqlite"
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
	if _, ok := h.zahtevajDozvolu(w, r, "kategorija.dodaj"); !ok {
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
		Kod:   normalizujKod(r.FormValue("kod")),
		Marza: parsirajMarzu(r.FormValue("marza")),
	}

	if _, err := h.KategorijeRepo.Kreiraj(r.Context(), k); err != nil {
		http.Error(w, "Greška pri čuvanju kategorije", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/magacin/kategorije?sacuvano=1", http.StatusSeeOther)
}

// IzmeniKategoriju prima POST i ažurira naziv, opis i maržu postojeće kategorije
func (h *Handler) IzmeniKategoriju(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "kategorija.izmeni"); !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Neispravan ID kategorije", http.StatusBadRequest)
		return
	}

	naziv := r.FormValue("naziv")
	if naziv == "" {
		http.Redirect(w, r, "/magacin/kategorije", http.StatusSeeOther)
		return
	}

	k := &model.Kategorija{
		ID:    id,
		Naziv: naziv,
		Opis:  r.FormValue("opis"),
		Kod:   normalizujKod(r.FormValue("kod")),
		Marza: parsirajMarzu(r.FormValue("marza")),
	}

	if err := h.KategorijeRepo.Izmeni(r.Context(), k); err != nil {
		http.Error(w, "Greška pri čuvanju izmene kategorije", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/magacin/kategorije?sacuvano=1", http.StatusSeeOther)
}

// parsirajMarzu pretvara tekst iz forme u *float64; prazno/neispravno → nil (NULL u bazi)
func parsirajMarzu(s string) *float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v < 0 {
		return nil
	}
	return &v
}

// normalizujKod čisti kôd kategorije za upotrebu kao prefiks šifre:
// velika slova, zadržava samo slova i brojeve (bez razmaka i specijalnih znakova).
func normalizujKod(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	kod := b.String()
	// USL i TRO su rezervisani prefiksi (usluge i troškovi) — ne dozvoljavamo ih
	// kao kôd kategorije da auto-šifra artikla ne bi pala u rezervisani prostor
	if kod == "USL" || kod == "TRO" {
		return ""
	}
	return kod
}

// ObrisiKategoriju briše kategoriju po ID-u
func (h *Handler) ObrisiKategoriju(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "kategorija.obrisi"); !ok {
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
