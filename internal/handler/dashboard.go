package handler

import (
	"log/slog"
	"net/http"
	"os"

	appdb "ntech/internal/db"
	"ntech/internal/db/sqlite"
	"ntech/internal/middleware"
	"ntech/internal/model"
)

// Dashboard renderuje početnu stranicu sa pravim podacima iz baze
func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// pročitaj i odmah obrišt flash poruku ako postoji
	var flashGreska string
	if kol, err := r.Cookie("ntech_flash_greska"); err == nil {
		flashGreska = kol.Value
		http.SetCookie(w, &http.Cookie{
			Name:     "ntech_flash_greska",
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   os.Getenv("NTECH_ENV") == "production",
		})
	}

	podesavanja, err := sqlite.DohvatiSvaPodesavanja(ctx, h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	var brojArtikala, aktivniServisi, kriticnaZaliha, aktivniPodsetnici int
	var prihodOvogMeseca float64

	if n, err := h.IzvestajRepo.BrojArtikala(ctx); err != nil {
		slog.Error("dashboard: broj artikala", "error", err)
	} else {
		brojArtikala = n
	}

	if n, err := h.IzvestajRepo.BrojAktivnihServisa(ctx); err != nil {
		slog.Error("dashboard: aktivni servisi", "error", err)
	} else {
		aktivniServisi = n
	}

	// prihod se dohvata samo ako korisnik ima dozvolu dashboard.prihod
	korisnikDash := middleware.KorisnikIzKonteksta(ctx)
	if h.DozvoleRepo.ImaDozvolu(ctx, korisnikDash.Uloga, "dashboard.prihod") {
		if v, err := h.IzvestajRepo.PrihodTekuciMesec(ctx); err != nil {
			slog.Error("dashboard: prihod ovog meseca", "error", err)
		} else {
			prihodOvogMeseca = v
		}
	}

	if n, err := h.IzvestajRepo.BrojKriticnihZaliha(ctx); err != nil {
		slog.Error("dashboard: kriticna zaliha", "error", err)
	} else {
		kriticnaZaliha = n
	}

	korisnikFilter := appdb.PodsetnikFilter{}
	if korisnikDash.Uloga == "radnik" {
		korisnikFilter.KorisnikID = &korisnikDash.ID
	}
	if n, err := h.PodsetnikRepo.BrojAktivnih(ctx, korisnikFilter); err != nil {
		slog.Error("dashboard: aktivni podsetnici", "error", err)
	} else {
		aktivniPodsetnici = n
	}

	// poslednjih 5 servisnih naloga — repo vraća sirove redove, ovde formatiramo
	var poslednjiServisi []model.StavkaServisa
	if redovi, err := h.IzvestajRepo.PoslednjiServisi(ctx, 5); err != nil {
		slog.Error("dashboard: poslednji servisi", "error", err)
	} else {
		for _, s := range redovi {
			poslednjiServisi = append(poslednjiServisi, model.StavkaServisa{
				Uredjaj:      s.Uredjaj,
				Status:       s.Status,
				BojaTacke:    bojaTackeServisa(s.Status),
				DatumPrijema: s.DatumPrijema.Format("02.01."),
			})
		}
	}

	// artikli sa kritičnom zalihom
	var kriticneZalihe []model.StavkaZalihe
	if redovi, err := h.IzvestajRepo.KriticneZalihe(ctx, 5); err != nil {
		slog.Error("dashboard: kriticne zalihe", "error", err)
	} else {
		for _, z := range redovi {
			boja := "#f97316"
			if z.Kolicina == 0 || (z.KolicinaMin > 0 && z.Kolicina <= z.KolicinaMin/2) {
				boja = "#dc2626"
			}
			kriticneZalihe = append(kriticneZalihe, model.StavkaZalihe{
				Naziv:     z.Naziv,
				Kolicina:  z.Kolicina,
				BojaTacke: boja,
			})
		}
	}

	// poslednjih 5 prodajnih naloga
	var poslednjeProdaje []model.StavkaProdajePregled
	if redovi, err := h.IzvestajRepo.PoslednjeProdaje(ctx, 5); err != nil {
		slog.Error("dashboard: poslednje prodaje", "error", err)
	} else {
		for _, p := range redovi {
			poslednjeProdaje = append(poslednjeProdaje, model.StavkaProdajePregled{
				BrojNaloga:   p.BrojNaloga,
				Ukupno:       p.Ukupno,
				Datum:        p.Datum.Format("02.01."),
				KlijentNaziv: p.KlijentNaziv,
			})
		}
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "dashboard"
	ps.NaslovStranice = "Dashboard"

	podaci := model.PodaciDashboarda{
		PodaciStranice: ps,
		BrojArtikala:      brojArtikala,
		AktivniServisi:    aktivniServisi,
		PrihodOvogMeseca:  prihodOvogMeseca,
		KriticnaZaliha:    kriticnaZaliha,
		AktivniPodsetnici: aktivniPodsetnici,
		PoslednjiServisi:  poslednjiServisi,
		KriticneZalihe:    kriticneZalihe,
		PoslednjeProdaje:  poslednjeProdaje,
		FlashGreska:       flashGreska,
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
