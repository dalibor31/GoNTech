package handler

import (
	"html/template"
	"log"
	"net/http"

	"ntech/internal/db/sqlite"
	"ntech/internal/model"
)

// Dashboard renderuje početnu stranicu sa pravim podacima iz baze
func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	podesavanja, err := sqlite.DohvatiSvaPodesavanja(ctx, h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	var brojArtikala, aktivniServisi, prodajaOvogMeseca, kriticnaZaliha int

	if err := h.DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM artikli",
	).Scan(&brojArtikala); err != nil {
		log.Printf("dashboard: broj artikala: %v", err)
	}

	if err := h.DB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM servisni_nalozi
		WHERE status NOT IN ('Završeno', 'Preuzeto')`,
	).Scan(&aktivniServisi); err != nil {
		log.Printf("dashboard: aktivni servisi: %v", err)
	}

	if err := h.DB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM prodajni_nalozi
		WHERE strftime('%Y-%m', datum) = strftime('%Y-%m', 'now', 'localtime')`,
	).Scan(&prodajaOvogMeseca); err != nil {
		log.Printf("dashboard: prodaja ovog meseca: %v", err)
	}

	if err := h.DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM artikli WHERE kolicina <= kolicina_min",
	).Scan(&kriticnaZaliha); err != nil {
		log.Printf("dashboard: kriticna zaliha: %v", err)
	}

	// poslednjih 5 servisnih naloga
	servisRedovi, err := h.DB.QueryContext(ctx, `
		SELECT uredjaj, status FROM servisni_nalozi
		ORDER BY datum_prijema DESC LIMIT 5`)
	if err != nil {
		log.Printf("dashboard: poslednji servisi: %v", err)
	}

	var poslednjiServisi []model.StavkaServisa
	if servisRedovi != nil {
		defer servisRedovi.Close()
		for servisRedovi.Next() {
			var s model.StavkaServisa
			if err := servisRedovi.Scan(&s.Uredjaj, &s.Status); err == nil {
				s.BojaTacke = bojaTackeServisa(s.Status)
				poslednjiServisi = append(poslednjiServisi, s)
			}
		}
	}

	// artikli sa kritičnom zalihom, sortirani po količini rastuće
	zaliheRedovi, err := h.DB.QueryContext(ctx, `
		SELECT naziv, kolicina FROM artikli
		WHERE kolicina <= kolicina_min
		ORDER BY kolicina ASC LIMIT 5`)
	if err != nil {
		log.Printf("dashboard: kriticne zalihe: %v", err)
	}

	var kriticneZalihe []model.StavkaZalihe
	if zaliheRedovi != nil {
		defer zaliheRedovi.Close()
		for zaliheRedovi.Next() {
			var z model.StavkaZalihe
			if err := zaliheRedovi.Scan(&z.Naziv, &z.Kolicina); err == nil {
				if z.Kolicina == 0 {
					z.BojaTacke = "#dc2626"
				} else {
					z.BojaTacke = "#f97316"
				}
				kriticneZalihe = append(kriticneZalihe, z)
			}
		}
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
		BrojArtikala:      brojArtikala,
		AktivniServisi:    aktivniServisi,
		ProdajaOvogMeseca: prodajaOvogMeseca,
		KriticnaZaliha:    kriticnaZaliha,
		PoslednjiServisi:  poslednjiServisi,
		KriticneZalihe:    kriticneZalihe,
	}

	tmpl, err := template.ParseFiles(
		"web/templates/teme/podrazumevana/base.html",
		"web/templates/komponente/sidebar.html",
		"web/templates/komponente/topbar.html",
		"web/templates/stranice/dashboard.html",
	)
	if err != nil {
		log.Printf("greška pri učitavanju šablona: %v", err)
		http.Error(w, "Greška pri učitavanju stranice", http.StatusInternalServerError)
		return
	}

	if err := tmpl.ExecuteTemplate(w, "base", podaci); err != nil {
		log.Printf("greška pri renderovanju: %v", err)
		http.Error(w, "Greška pri prikazu stranice", http.StatusInternalServerError)
	}
}

// bojaTackeServisa vraća hex boju tačke prema statusu naloga
func bojaTackeServisa(status string) string {
	switch status {
	case "U dijagnostici":
		return "#3b82f6"
	case "Čeka delove":
		return "#f97316"
	case "U popravci":
		return "#ca8a04"
	case "Završeno":
		return "#16a34a"
	case "Preuzeto":
		return "#15803d"
	default:
		return "#94a3b8"
	}
}
