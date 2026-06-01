package handler

import (
	"html/template"
	"log"
	"net/http"

	"ntech/internal/db/sqlite"
	"ntech/internal/model"
)

// Dashboard renderuje početnu stranicu
func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	// čitamo sva podešavanja iz baze
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	podaci := model.PodaciDashboarda{
		PodaciStranice: model.PodaciStranice{
			Stranica:       "dashboard",
			NaslovStranice: "Dashboard",
			Tema:           podesavanja["tema"],
			NazivFirme:     podesavanja["naziv_firme"],
			Podnazlov:      podesavanja["podnazlov"],
			LogoTip:        podesavanja["logo_tip"],
			LogoPutanja:    podesavanja["logo_putanja"],
			Korisnik:       "Admin",
		},
		BrojArtikala:      0,
		AktivniServisi:    0,
		ProdajaOvogMeseca: 0,
		KriticnaZaliha:    0,
		PoslednjiServisi:  []model.StavkaServisa{},
		KriticneZalihe:    []model.StavkaZalihe{},
	}

	tmpl, err := template.ParseFiles(
		"web/templates/teme/podrazumevana/base.html",
		"web/templates/komponente/sidebar.html",
		"web/templates/komponente/topbar.html",
		"web/templates/stranice/dashboard.html",
	)
	if err != nil {
		http.Error(w, "Greška pri učitavanju stranice", http.StatusInternalServerError)
		return
	}

	if err := tmpl.ExecuteTemplate(w, "base", podaci); err != nil {
		log.Printf("greška pri renderovanju: %v", err)
		http.Error(w, "Greška pri prikazu stranice", http.StatusInternalServerError)
		return
	}
}
