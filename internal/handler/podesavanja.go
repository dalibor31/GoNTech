package handler

import (
	"html/template"
	"net/http"

	"ntech/internal/db/sqlite"
	"ntech/internal/model"
)

// PodaciPodesavanja su podaci za stranicu podešavanja
type PodaciPodesavanja struct {
	model.PodaciStranice
	NazivFirme  string
	Podnazlov   string
	LogoTip     string
	LogoPutanja string
	Tema        string
	Sacuvano    bool
}

// Podesavanja renderuje stranicu podešavanja
func (h *Handler) Podesavanja(w http.ResponseWriter, r *http.Request) {
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	podaci := PodaciPodesavanja{
		PodaciStranice: model.PodaciStranice{
			Stranica:       "podesavanja",
			NaslovStranice: "Podešavanja",
			Tema:           podesavanja["tema"],
			NazivFirme:     podesavanja["naziv_firme"],
			Podnazlov:      podesavanja["podnazlov"],
			LogoTip:        podesavanja["logo_tip"],
			LogoPutanja:    podesavanja["logo_putanja"],
			Korisnik:       "Admin",
		},
		NazivFirme:  podesavanja["naziv_firme"],
		Podnazlov:   podesavanja["podnazlov"],
		LogoTip:     podesavanja["logo_tip"],
		LogoPutanja: podesavanja["logo_putanja"],
		Tema:        podesavanja["tema"],
		Sacuvano:    r.URL.Query().Get("sacuvano") == "1",
	}

	tmpl, err := template.ParseFiles(
		"web/templates/teme/podrazumevana/base.html",
		"web/templates/komponente/sidebar.html",
		"web/templates/komponente/topbar.html",
		"web/templates/stranice/podesavanja.html",
	)
	if err != nil {
		http.Error(w, "Greška pri učitavanju stranice", http.StatusInternalServerError)
		return
	}

	if err := tmpl.ExecuteTemplate(w, "base", podaci); err != nil {
		http.Error(w, "Greška pri prikazu stranice", http.StatusInternalServerError)
		return
	}
}

// SacuvajPodesavanja prima POST i čuva podešavanja u bazu
func (h *Handler) SacuvajPodesavanja(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	polja := map[string]string{
		"naziv_firme": r.FormValue("naziv_firme"),
		"podnazlov":   r.FormValue("podnazlov"),
		"logo_tip":    r.FormValue("logo_tip"),
		"tema":        r.FormValue("tema"),
	}

	for kljuc, vrednost := range polja {
		if vrednost == "" {
			continue
		}
		if err := sqlite.SacuvajPodesavanje(r.Context(), h.DB, kljuc, vrednost); err != nil {
			http.Error(w, "Greška pri čuvanju podešavanja", http.StatusInternalServerError)
			return
		}
	}

	http.Redirect(w, r, "/podesavanja?sacuvano=1", http.StatusSeeOther)
}
