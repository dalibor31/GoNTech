package handler

import (
	"html/template"
	"net/http"

	"ntech/internal/model"
)

// Dashboard renderuje početnu stranicu
func Dashboard(w http.ResponseWriter, r *http.Request) {
	// za sad koristimo testne podatke — kasnije će ići iz baze
	podaci := model.PodaciDashboarda{
		PodaciStranice: model.PodaciStranice{
			Stranica:       "dashboard",
			NaslovStranice: "Dashboard",
			Tema:           "tamna",
			NazivFirme:     "NTech",
			Korisnik:       "Admin",
		},
		BrojArtikala:      0,
		AktivniServisi:    0,
		ProdajaOvogMeseca: 0,
		KriticnaZaliha:    0,
		PoslednjiServisi:  []model.StavkaServisa{},
		KriticneZalihe:    []model.StavkaZalihe{},
	}

	// učitavamo sve potrebne šablone zajedno
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

	// renderujemo base šablon sa podacima
	if err := tmpl.ExecuteTemplate(w, "base", podaci); err != nil {
		http.Error(w, "Greška pri prikazu stranice", http.StatusInternalServerError)
		return
	}
}
