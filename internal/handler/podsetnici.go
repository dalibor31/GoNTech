package handler

import (
	"log"
	"net/http"
	"strings"
	"time"

	"ntech/internal/db"
	"ntech/internal/db/sqlite"
	"ntech/internal/model"

	"github.com/go-chi/chi/v5"
)

// podaciPodsetnici su podaci za stranicu sa listom podsetnika
type podaciPodsetnici struct {
	model.PodaciStranice
	Podsetnici  []model.Podsetnik
	SamoAktivni bool
	Sacuvano    bool
	Obrisan     bool
}

// podaciPodsetnikForma su podaci za formu novog/izmenjenog podsetnika
type podaciPodsetnikForma struct {
	model.PodaciStranice
	Podsetnik model.Podsetnik
	Greska    string
	Izmena    bool
}

// Podsetnici renderuje listu podsetnika
func (h *Handler) Podsetnici(w http.ResponseWriter, r *http.Request) {
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	samoAktivni := r.URL.Query().Get("samo_aktivni") == "1"

	lista, err := h.PodsetniciFRepo.Lista(r.Context(), db.PodsetnikFilter{SamoAktivni: samoAktivni})
	if err != nil {
		http.Error(w, "Greška pri učitavanju podsetnika", http.StatusInternalServerError)
		return
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "podsetnici"
	ps.NaslovStranice = "Podsetnici"

	podaci := podaciPodsetnici{
		PodaciStranice: ps,
		Podsetnici:     lista,
		SamoAktivni:    samoAktivni,
		Sacuvano:       r.URL.Query().Get("sacuvano") == "1",
		Obrisan:        r.URL.Query().Get("obrisan") == "1",
	}

	h.renderujTemplate(w, "podsetnici", podaci)
}

// NoviPodsetnik prikazuje praznu formu za unos novog podsetnika
func (h *Handler) NoviPodsetnik(w http.ResponseWriter, r *http.Request) {
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "podsetnici"
	ps.NaslovStranice = "Novi podsetnik"

	h.renderujFormuPodsetnika(w, podaciPodsetnikForma{
		PodaciStranice: ps,
		Izmena:         false,
	})
}

// SacuvajPodsetnik prima POST formu i upisuje novi podsetnik u bazu
func (h *Handler) SacuvajPodsetnik(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	podsetnik, greska := parseFormuPodsetnika(r)
	if greska != "" {
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		ps := h.popuniPodaciStranice(r, podesavanja)
		ps.Stranica = "podsetnici"
		ps.NaslovStranice = "Novi podsetnik"
		h.renderujFormuPodsetnika(w, podaciPodsetnikForma{
			PodaciStranice: ps,
			Podsetnik:      podsetnik,
			Greska:         greska,
			Izmena:         false,
		})
		return
	}

	if _, err := h.PodsetniciFRepo.Kreiraj(r.Context(), &podsetnik); err != nil {
		log.Printf("greška pri čuvanju podsetnika: %v", err)
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		ps := h.popuniPodaciStranice(r, podesavanja)
		ps.Stranica = "podsetnici"
		ps.NaslovStranice = "Novi podsetnik"
		h.renderujFormuPodsetnika(w, podaciPodsetnikForma{
			PodaciStranice: ps,
			Podsetnik:      podsetnik,
			Greska:         "Došlo je do greške pri čuvanju. Pokušajte ponovo.",
			Izmena:         false,
		})
		return
	}

	http.Redirect(w, r, "/podsetnici?sacuvano=1", http.StatusSeeOther)
}

// IzmeniPodsetnik učitava podsetnik po ID-u i prikazuje popunjenu formu za izmenu
func (h *Handler) IzmeniPodsetnik(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID podsetnika", http.StatusBadRequest)
		return
	}

	podsetnik, err := h.PodsetniciFRepo.DohvatiID(r.Context(), id)
	if err != nil {
		http.Error(w, "Podsetnik nije pronađen", http.StatusNotFound)
		return
	}

	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "podsetnici"
	ps.NaslovStranice = "Izmeni podsetnik"

	h.renderujFormuPodsetnika(w, podaciPodsetnikForma{
		PodaciStranice: ps,
		Podsetnik:      *podsetnik,
		Izmena:         true,
	})
}

// SacuvajIzmenePodsetnika prima POST formu i ažurira postojeći podsetnik u bazi
func (h *Handler) SacuvajIzmenePodsetnika(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID podsetnika", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	podsetnik, greska := parseFormuPodsetnika(r)
	podsetnik.ID = id
	if greska != "" {
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		ps := h.popuniPodaciStranice(r, podesavanja)
		ps.Stranica = "podsetnici"
		ps.NaslovStranice = "Izmeni podsetnik"
		h.renderujFormuPodsetnika(w, podaciPodsetnikForma{
			PodaciStranice: ps,
			Podsetnik:      podsetnik,
			Greska:         greska,
			Izmena:         true,
		})
		return
	}

	if err := h.PodsetniciFRepo.Izmeni(r.Context(), &podsetnik); err != nil {
		log.Printf("greška pri čuvanju izmene podsetnika: %v", err)
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		ps := h.popuniPodaciStranice(r, podesavanja)
		ps.Stranica = "podsetnici"
		ps.NaslovStranice = "Izmeni podsetnik"
		h.renderujFormuPodsetnika(w, podaciPodsetnikForma{
			PodaciStranice: ps,
			Podsetnik:      podsetnik,
			Greska:         "Došlo je do greške pri čuvanju. Pokušajte ponovo.",
			Izmena:         true,
		})
		return
	}

	http.Redirect(w, r, "/podsetnici?sacuvano=1", http.StatusSeeOther)
}

// OznaciPodsetnik prima POST zahtev i menja status završenosti podsetnika
func (h *Handler) OznaciPodsetnik(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID podsetnika", http.StatusBadRequest)
		return
	}

	// učitamo trenutni status pa ga preokrenemo
	podsetnik, err := h.PodsetniciFRepo.DohvatiID(r.Context(), id)
	if err != nil {
		http.Error(w, "Podsetnik nije pronađen", http.StatusNotFound)
		return
	}

	if err := h.PodsetniciFRepo.OznaciZavrsenim(r.Context(), id, !podsetnik.Zavrseno); err != nil {
		http.Error(w, "Greška pri ažuriranju statusa", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/podsetnici", http.StatusSeeOther)
}

// ObrisiPodsetnik prima POST zahtev i briše podsetnik po ID-u
func (h *Handler) ObrisiPodsetnik(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID podsetnika", http.StatusBadRequest)
		return
	}

	if err := h.PodsetniciFRepo.Obrisi(r.Context(), id); err != nil {
		http.Error(w, "Greška pri brisanju podsetnika", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/podsetnici?obrisan=1", http.StatusSeeOther)
}

// parseFormuPodsetnika čita polja iz HTTP forme, validira ih i vraća model i eventualnu grešku
func parseFormuPodsetnika(r *http.Request) (model.Podsetnik, string) {
	naslov := strings.TrimSpace(r.FormValue("naslov"))
	if naslov == "" {
		return model.Podsetnik{}, "Naslov je obavezan."
	}

	datumStr := strings.TrimSpace(r.FormValue("datum_podsecanja"))
	if datumStr == "" {
		return model.Podsetnik{Naslov: naslov}, "Datum podsećanja je obavezan."
	}

	datum, err := time.Parse("2006-01-02", datumStr)
	if err != nil {
		return model.Podsetnik{Naslov: naslov}, "Datum podsećanja nije u ispravnom formatu."
	}

	return model.Podsetnik{
		Naslov:          naslov,
		Napomena:        strings.TrimSpace(r.FormValue("napomena")),
		DatumPodsecanja: datum,
		Tip:             model.TipOpsti,
	}, ""
}

// renderujFormuPodsetnika renderuje HTML šablon forme za unos ili izmenu podsetnika
func (h *Handler) renderujFormuPodsetnika(w http.ResponseWriter, podaci podaciPodsetnikForma) {
	h.renderujTemplate(w, "podsetnik_forma", podaci)
}
