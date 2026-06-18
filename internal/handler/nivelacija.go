package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ntech/internal/db/sqlite"
	"ntech/internal/middleware"
	"ntech/internal/model"

	"github.com/go-chi/chi/v5"
)

// PodaciNivelacije su podaci za pregled istorije promene cena
type PodaciNivelacije struct {
	model.PodaciStranice
	Zapisi []model.Nivelacija
	Od     string
	Do     string
}

// Nivelacije renderuje istoriju promena prodajnih cena za izabrani period
func (h *Handler) Nivelacije(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "artikal.izmeni"); !ok {
		return
	}
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	odStr := r.URL.Query().Get("od")
	doStr := r.URL.Query().Get("do")
	// podrazumevano: tekući mesec (od prvog do poslednjeg dana)
	if odStr == "" || doStr == "" {
		sada := time.Now()
		prvi := time.Date(sada.Year(), sada.Month(), 1, 0, 0, 0, 0, sada.Location())
		poslednji := prvi.AddDate(0, 1, -1)
		if odStr == "" {
			odStr = prvi.Format("2006-01-02")
		}
		if doStr == "" {
			doStr = poslednji.Format("2006-01-02")
		}
	}
	zapisi, err := h.NivelacijaRepo.Lista(r.Context(), parsiraDatumOpcionalno(odStr), parsiraDatumOpcionalno(doStr))
	if err != nil {
		http.Error(w, "Greška pri učitavanju nivelacija", http.StatusInternalServerError)
		return
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "nivelacije"
	ps.NaslovStranice = "Nivelacije — promene cena"
	h.renderujTemplate(w, "nivelacije", PodaciNivelacije{
		PodaciStranice: ps,
		Zapisi:         zapisi,
		Od:             odStr,
		Do:             doStr,
	})
}

// PromeniCenuArtikla menja prodajnu cenu artikla i upisuje nivelacioni zapis (izvor "rucno").
func (h *Handler) PromeniCenuArtikla(w http.ResponseWriter, r *http.Request) {
	k, ok := h.zahtevajDozvolu(w, r, "artikal.izmeni")
	if !ok {
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Neispravan ID artikla", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	novaCena := parsiraIznos(r.FormValue("nova_cena"))
	razlog := strings.TrimSpace(r.FormValue("razlog"))
	if novaCena <= 0 {
		middleware.SetFlash(w, r, h.DB, "greska", "Nova cena mora biti veća od nule.")
		http.Redirect(w, r, "/magacin", http.StatusSeeOther)
		return
	}

	korisnikID := &k.ID
	_, err = h.NivelacijaRepo.PromeniCenu(r.Context(), id, novaCena, razlog, korisnikID)
	switch {
	case errors.Is(err, sqlite.ErrArtikalNePostoji):
		middleware.SetFlash(w, r, h.DB, "greska", "Artikal nije pronađen.")
	case err != nil:
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri promeni cene.")
	default:
		middleware.SetFlash(w, r, h.DB, "uspeh", "Prodajna cena je izmenjena.")
	}
	http.Redirect(w, r, "/magacin", http.StatusSeeOther)
}
