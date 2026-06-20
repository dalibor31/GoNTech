package handler

import (
	"net/http"
	"strconv"
	"strings"

	"ntech/internal/db/sqlite"
	"ntech/internal/middleware"
	"ntech/internal/model"

	"github.com/go-chi/chi/v5"
)

// validneOznakeStope su dozvoljene oznake PDV stope (vrsta po zakonu)
var validneOznakeStope = map[string]bool{
	"opsta":       true,
	"posebna":     true,
	"oslobodjeno": true,
}

// PodaciPdvStope su podaci za stranicu „Kalkulacija i PDV" (marža + šifarnik PDV stopa)
type PodaciPdvStope struct {
	model.PodaciStranice
	Stope            []model.PdvStopa
	KalkulacijaMarza string // podrazumevana marža (%) za kalkulaciju
}

// PdvStope renderuje stranicu „Kalkulacija i PDV": podešavanje podrazumevane marže
// i šifarnik PDV stopa (sve stope, uključujući arhivirane)
func (h *Handler) PdvStope(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "podesavanja.pregled"); !ok {
		return
	}
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}
	stope, err := h.PdvStopeRepo.Lista(r.Context(), false)
	if err != nil {
		http.Error(w, "Greška pri učitavanju PDV stopa", http.StatusInternalServerError)
		return
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "podesavanja-kalkulacija-pdv"
	ps.NaslovStranice = "Kalkulacija i PDV"
	h.renderujTemplate(w, "pdv_stope", PodaciPdvStope{
		PodaciStranice:   ps,
		Stope:            stope,
		KalkulacijaMarza: vrednostIliDefault(podesavanja, "kalkulacija_marza", "20"),
	})
}

// parsePdvStopuForma čita i proverava polja forme; vraća popunjenu stopu i poruku o grešci
func parsePdvStopuForma(r *http.Request) (model.PdvStopa, string) {
	naziv := strings.TrimSpace(r.FormValue("naziv"))
	oznaka := strings.TrimSpace(r.FormValue("oznaka"))
	stopaTekst := strings.TrimSpace(strings.Replace(r.FormValue("stopa"), ",", ".", 1))
	redosled, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("redosled")))

	if naziv == "" {
		return model.PdvStopa{}, "Naziv stope je obavezan."
	}
	if !validneOznakeStope[oznaka] {
		return model.PdvStopa{}, "Oznaka mora biti opšta, posebna ili oslobođeno."
	}
	stopa, err := strconv.ParseFloat(stopaTekst, 64)
	if err != nil || stopa < 0 || stopa > 100 {
		return model.PdvStopa{}, "Stopa mora biti broj između 0 i 100."
	}

	return model.PdvStopa{
		Naziv:    naziv,
		Stopa:    stopa,
		Oznaka:   oznaka,
		Aktivna:  true,
		Redosled: redosled,
	}, ""
}

// DodajPdvStopu prima POST i upisuje novu stopu u šifarnik
func (h *Handler) DodajPdvStopu(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "podesavanja.izmeni"); !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}
	stopa, greska := parsePdvStopuForma(r)
	if greska != "" {
		middleware.SetFlash(w, r, h.DB, "greska", greska)
		http.Redirect(w, r, "/admin/podesavanja/kalkulacija-pdv", http.StatusSeeOther)
		return
	}
	if _, err := h.PdvStopeRepo.Kreiraj(r.Context(), &stopa); err != nil {
		http.Error(w, "Greška pri čuvanju PDV stope", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/podesavanja/kalkulacija-pdv?sacuvano=1", http.StatusSeeOther)
}

// IzmeniPdvStopu prima POST i menja postojeću stopu
func (h *Handler) IzmeniPdvStopu(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "podesavanja.izmeni"); !ok {
		return
	}
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID stope", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}
	stopa, greska := parsePdvStopuForma(r)
	if greska != "" {
		middleware.SetFlash(w, r, h.DB, "greska", greska)
		http.Redirect(w, r, "/admin/podesavanja/kalkulacija-pdv", http.StatusSeeOther)
		return
	}
	stopa.ID = id
	if err := h.PdvStopeRepo.Izmeni(r.Context(), &stopa); err != nil {
		http.Error(w, "Greška pri izmeni PDV stope", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/podesavanja/kalkulacija-pdv?sacuvano=1", http.StatusSeeOther)
}

// PromeniAktivnostPdvStope arhivira ili vraća stopu u upotrebu (toggle, bez brisanja)
func (h *Handler) PromeniAktivnostPdvStope(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "podesavanja.izmeni"); !ok {
		return
	}
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID stope", http.StatusBadRequest)
		return
	}
	postojeca, err := h.PdvStopeRepo.DohvatiID(r.Context(), id)
	if err != nil {
		http.Error(w, "PDV stopa nije pronađena", http.StatusNotFound)
		return
	}
	if err := h.PdvStopeRepo.PostaviAktivnu(r.Context(), id, !postojeca.Aktivna); err != nil {
		http.Error(w, "Greška pri promeni statusa PDV stope", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/podesavanja/kalkulacija-pdv?sacuvano=1", http.StatusSeeOther)
}
