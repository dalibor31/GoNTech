package handler

import (
	"log"
	"net/http"
	"time"

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

	var brojArtikala, aktivniServisi, kriticnaZaliha, aktivniPodsetnici int
	var prihodOvogMeseca float64

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
		SELECT COALESCE(SUM(ukupno), 0) FROM prodajni_nalozi
		WHERE substr(datum, 1, 7) = strftime('%Y-%m', 'now', 'localtime')`,
	).Scan(&prihodOvogMeseca); err != nil {
		log.Printf("dashboard: prihod ovog meseca: %v", err)
	}

	if err := h.DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM artikli WHERE kolicina <= kolicina_min",
	).Scan(&kriticnaZaliha); err != nil {
		log.Printf("dashboard: kriticna zaliha: %v", err)
	}

	if n, err := h.PodsetniciFRepo.BrojAktivnih(ctx); err != nil {
		log.Printf("dashboard: aktivni podsetnici: %v", err)
	} else {
		aktivniPodsetnici = n
	}

	// poslednjih 5 servisnih naloga sa datumom prijema
	servisRedovi, err := h.DB.QueryContext(ctx, `
		SELECT uredjaj, status, datum_prijema FROM servisni_nalozi
		ORDER BY datum_prijema DESC LIMIT 5`)
	if err != nil {
		log.Printf("dashboard: poslednji servisi: %v", err)
	}

	var poslednjiServisi []model.StavkaServisa
	if servisRedovi != nil {
		defer servisRedovi.Close()
		for servisRedovi.Next() {
			var s model.StavkaServisa
			var datum time.Time
			if err := servisRedovi.Scan(&s.Uredjaj, &s.Status, &datum); err == nil {
				s.BojaTacke = bojaTackeServisa(s.Status)
				s.DatumPrijema = datum.Format("02.01.")
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

	// poslednjih 5 prodajnih naloga sa nazivom klijenta
	prodajaRedovi, err := h.DB.QueryContext(ctx, `
		SELECT
			pn.broj_naloga, pn.ukupno, pn.datum,
			COALESCE(NULLIF(k.naziv_firme, ''), TRIM(COALESCE(k.ime, '') || ' ' || COALESCE(k.prezime, '')), '') AS klijent_naziv
		FROM prodajni_nalozi pn
		LEFT JOIN klijenti k ON k.id = pn.klijent_id
		ORDER BY pn.datum DESC LIMIT 5`)
	if err != nil {
		log.Printf("dashboard: poslednje prodaje: %v", err)
	}

	var poslednjeProdaje []model.StavkaProdajePregled
	if prodajaRedovi != nil {
		defer prodajaRedovi.Close()
		for prodajaRedovi.Next() {
			var p model.StavkaProdajePregled
			var datum time.Time
			if err := prodajaRedovi.Scan(&p.BrojNaloga, &p.Ukupno, &datum, &p.KlijentNaziv); err == nil {
				p.Datum = datum.Format("02.01.")
				poslednjeProdaje = append(poslednjeProdaje, p)
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
		PrihodOvogMeseca:  prihodOvogMeseca,
		KriticnaZaliha:    kriticnaZaliha,
		AktivniPodsetnici: aktivniPodsetnici,
		PoslednjiServisi:  poslednjiServisi,
		KriticneZalihe:    kriticneZalihe,
		PoslednjeProdaje:  poslednjeProdaje,
	}

	h.renderujTemplate(w, "dashboard", podaci)
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
