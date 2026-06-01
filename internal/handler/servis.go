package handler

import (
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ntech/internal/db/sqlite"
	"ntech/internal/model"

	"github.com/go-chi/chi/v5"
)

// PodaciServisa su podaci za stranicu sa listom servisnih naloga
type PodaciServisa struct {
	model.PodaciStranice
	Nalozi       []model.ServisniNalogSaKlijentom
	Pretraga     string
	FilterStatus string
	SviStatusi   []string
	Sacuvano     bool
	Obrisan      bool
}

// PodaciFormeNaloga su podaci za formu novog/izmenjenog servisnog naloga
type PodaciFormeNaloga struct {
	model.PodaciStranice
	Nalog      model.ServisniNalog
	Klijenti   []model.Klijent
	SviStatusi []string
	Greska     string
	Izmena     bool
}

// PodaciDetaljiNaloga su podaci za pregled jednog servisnog naloga
type PodaciDetaljiNaloga struct {
	model.PodaciStranice
	Nalog        model.ServisniNalog
	KlijentNaziv string
	Sacuvano     bool
}

// Servis renderuje listu servisnih naloga sa opcionom pretragom i filterom statusa
func (h *Handler) Servis(w http.ResponseWriter, r *http.Request) {
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	pretraga := r.URL.Query().Get("pretraga")
	filterStatus := r.URL.Query().Get("status")

	nalozi, err := h.ServisRepo.Lista(r.Context(), pretraga, filterStatus)
	if err != nil {
		http.Error(w, "Greška pri učitavanju naloga", http.StatusInternalServerError)
		return
	}

	podaci := PodaciServisa{
		PodaciStranice: model.PodaciStranice{
			Stranica:       "servis",
			NaslovStranice: "Servis",
			Tema:           podesavanja["tema"],
			NazivFirme:     podesavanja["naziv_firme"],
			Podnazlov:      podesavanja["podnazlov"],
			LogoTip:        podesavanja["logo_tip"],
			LogoPutanja:    podesavanja["logo_putanja"],
			Korisnik:       "Admin",
		},
		Nalozi:       nalozi,
		Pretraga:     pretraga,
		FilterStatus: filterStatus,
		SviStatusi:   model.SviStatusi,
		Sacuvano:     r.URL.Query().Get("sacuvano") == "1",
		Obrisan:      r.URL.Query().Get("obrisan") == "1",
	}

	tmpl, err := template.ParseFiles(
		"web/templates/teme/podrazumevana/base.html",
		"web/templates/komponente/sidebar.html",
		"web/templates/komponente/topbar.html",
		"web/templates/stranice/servis.html",
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

// NoviNalog generiše broj naloga i prikazuje praznu formu za unos
func (h *Handler) NoviNalog(w http.ResponseWriter, r *http.Request) {
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	brojNaloga, err := h.ServisRepo.SledeciBroj(r.Context())
	if err != nil {
		http.Error(w, "Greška pri generisanju broja naloga", http.StatusInternalServerError)
		return
	}

	klijenti, err := h.KlijentiRepo.Lista(r.Context(), "")
	if err != nil {
		http.Error(w, "Greška pri učitavanju klijenata", http.StatusInternalServerError)
		return
	}

	renderujFormuNaloga(w, PodaciFormeNaloga{
		PodaciStranice: model.PodaciStranice{
			Stranica:       "servis",
			NaslovStranice: "Novi nalog",
			Tema:           podesavanja["tema"],
			NazivFirme:     podesavanja["naziv_firme"],
			Podnazlov:      podesavanja["podnazlov"],
			LogoTip:        podesavanja["logo_tip"],
			LogoPutanja:    podesavanja["logo_putanja"],
			Korisnik:       "Admin",
		},
		Nalog:      model.ServisniNalog{BrojNaloga: brojNaloga, Status: model.StatusPrimljeno},
		Klijenti:   klijenti,
		SviStatusi: model.SviStatusi,
		Izmena:     false,
	})
}

// SacuvajNalog prima POST formu i upisuje novi servisni nalog u bazu
func (h *Handler) SacuvajNalog(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	nalog, greska := parseFormuNaloga(r)
	if greska != "" {
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		klijenti, _ := h.KlijentiRepo.Lista(r.Context(), "")
		renderujFormuNaloga(w, PodaciFormeNaloga{
			PodaciStranice: model.PodaciStranice{
				Stranica:       "servis",
				NaslovStranice: "Novi nalog",
				Tema:           podesavanja["tema"],
				NazivFirme:     podesavanja["naziv_firme"],
				Podnazlov:      podesavanja["podnazlov"],
				LogoTip:        podesavanja["logo_tip"],
				LogoPutanja:    podesavanja["logo_putanja"],
				Korisnik:       "Admin",
			},
			Nalog:      nalog,
			Klijenti:   klijenti,
			SviStatusi: model.SviStatusi,
			Greska:     greska,
			Izmena:     false,
		})
		return
	}

	id, err := h.ServisRepo.Kreiraj(r.Context(), &nalog)
	if err != nil {
		log.Printf("greška pri čuvanju naloga: %v", err)
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		klijenti, _ := h.KlijentiRepo.Lista(r.Context(), "")
		renderujFormuNaloga(w, PodaciFormeNaloga{
			PodaciStranice: model.PodaciStranice{
				Stranica:       "servis",
				NaslovStranice: "Novi nalog",
				Tema:           podesavanja["tema"],
				NazivFirme:     podesavanja["naziv_firme"],
				Podnazlov:      podesavanja["podnazlov"],
				LogoTip:        podesavanja["logo_tip"],
				LogoPutanja:    podesavanja["logo_putanja"],
				Korisnik:       "Admin",
			},
			Nalog:      nalog,
			Klijenti:   klijenti,
			SviStatusi: model.SviStatusi,
			Greska:     "Došlo je do greške pri čuvanju. Pokušajte ponovo.",
			Izmena:     false,
		})
		return
	}

	http.Redirect(w, r, "/servis/"+strconv.FormatInt(id, 10)+"?sacuvano=1", http.StatusSeeOther)
}

// IzmeniNalog učitava servisni nalog po ID-u i prikazuje popunjenu formu
func (h *Handler) IzmeniNalog(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID naloga", http.StatusBadRequest)
		return
	}

	nalog, err := h.ServisRepo.DohvatiID(r.Context(), id)
	if err != nil {
		http.Error(w, "Nalog nije pronađen", http.StatusNotFound)
		return
	}

	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	klijenti, err := h.KlijentiRepo.Lista(r.Context(), "")
	if err != nil {
		http.Error(w, "Greška pri učitavanju klijenata", http.StatusInternalServerError)
		return
	}

	renderujFormuNaloga(w, PodaciFormeNaloga{
		PodaciStranice: model.PodaciStranice{
			Stranica:       "servis",
			NaslovStranice: "Izmeni nalog",
			Tema:           podesavanja["tema"],
			NazivFirme:     podesavanja["naziv_firme"],
			Podnazlov:      podesavanja["podnazlov"],
			LogoTip:        podesavanja["logo_tip"],
			LogoPutanja:    podesavanja["logo_putanja"],
			Korisnik:       "Admin",
		},
		Nalog:      *nalog,
		Klijenti:   klijenti,
		SviStatusi: model.SviStatusi,
		Izmena:     true,
	})
}

// SacuvajIzmenaNaloga prima POST formu i ažurira postojeći servisni nalog
func (h *Handler) SacuvajIzmenaNaloga(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID naloga", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	nalog, greska := parseFormuNaloga(r)
	if greska != "" {
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		klijenti, _ := h.KlijentiRepo.Lista(r.Context(), "")
		nalog.ID = id
		renderujFormuNaloga(w, PodaciFormeNaloga{
			PodaciStranice: model.PodaciStranice{
				Stranica:       "servis",
				NaslovStranice: "Izmeni nalog",
				Tema:           podesavanja["tema"],
				NazivFirme:     podesavanja["naziv_firme"],
				Podnazlov:      podesavanja["podnazlov"],
				LogoTip:        podesavanja["logo_tip"],
				LogoPutanja:    podesavanja["logo_putanja"],
				Korisnik:       "Admin",
			},
			Nalog:      nalog,
			Klijenti:   klijenti,
			SviStatusi: model.SviStatusi,
			Greska:     greska,
			Izmena:     true,
		})
		return
	}

	nalog.ID = id
	if err := h.ServisRepo.Izmeni(r.Context(), &nalog); err != nil {
		log.Printf("greška pri čuvanju izmene naloga: %v", err)
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		klijenti, _ := h.KlijentiRepo.Lista(r.Context(), "")
		renderujFormuNaloga(w, PodaciFormeNaloga{
			PodaciStranice: model.PodaciStranice{
				Stranica:       "servis",
				NaslovStranice: "Izmeni nalog",
				Tema:           podesavanja["tema"],
				NazivFirme:     podesavanja["naziv_firme"],
				Podnazlov:      podesavanja["podnazlov"],
				LogoTip:        podesavanja["logo_tip"],
				LogoPutanja:    podesavanja["logo_putanja"],
				Korisnik:       "Admin",
			},
			Nalog:      nalog,
			Klijenti:   klijenti,
			SviStatusi: model.SviStatusi,
			Greska:     "Došlo je do greške pri čuvanju. Pokušajte ponovo.",
			Izmena:     true,
		})
		return
	}

	http.Redirect(w, r, "/servis/"+strconv.FormatInt(id, 10)+"?sacuvano=1", http.StatusSeeOther)
}

// ObrisiNalog prima POST zahtev i briše servisni nalog po ID-u
func (h *Handler) ObrisiNalog(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID naloga", http.StatusBadRequest)
		return
	}

	if err := h.ServisRepo.Obrisi(r.Context(), id); err != nil {
		http.Error(w, "Greška pri brisanju naloga", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/servis?obrisan=1", http.StatusSeeOther)
}

// DetaljiNaloga prikazuje sve podatke jednog servisnog naloga
func (h *Handler) DetaljiNaloga(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID naloga", http.StatusBadRequest)
		return
	}

	nalog, err := h.ServisRepo.DohvatiID(r.Context(), id)
	if err != nil {
		http.Error(w, "Nalog nije pronađen", http.StatusNotFound)
		return
	}

	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	klijentNaziv := ""
	if nalog.KlijentID != nil {
		klijent, err := h.KlijentiRepo.DohvatiID(r.Context(), *nalog.KlijentID)
		if err == nil {
			if klijent.NazivFirme != "" {
				klijentNaziv = klijent.NazivFirme
			} else {
				klijentNaziv = strings.TrimSpace(klijent.Ime + " " + klijent.Prezime)
			}
		}
	}

	podaci := PodaciDetaljiNaloga{
		PodaciStranice: model.PodaciStranice{
			Stranica:       "servis",
			NaslovStranice: "Detalji naloga",
			Tema:           podesavanja["tema"],
			NazivFirme:     podesavanja["naziv_firme"],
			Podnazlov:      podesavanja["podnazlov"],
			LogoTip:        podesavanja["logo_tip"],
			LogoPutanja:    podesavanja["logo_putanja"],
			Korisnik:       "Admin",
		},
		Nalog:        *nalog,
		KlijentNaziv: klijentNaziv,
		Sacuvano:     r.URL.Query().Get("sacuvano") == "1",
	}

	tmpl, err := template.ParseFiles(
		"web/templates/teme/podrazumevana/base.html",
		"web/templates/komponente/sidebar.html",
		"web/templates/komponente/topbar.html",
		"web/templates/stranice/servis_detalji.html",
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

// parseFormuNaloga čita i validira polja iz HTTP forme
func parseFormuNaloga(r *http.Request) (model.ServisniNalog, string) {
	uredjaj := strings.TrimSpace(r.FormValue("uredjaj"))
	if uredjaj == "" {
		return model.ServisniNalog{}, "Naziv uređaja je obavezan."
	}

	opisKvara := strings.TrimSpace(r.FormValue("opis_kvara"))
	if opisKvara == "" {
		return model.ServisniNalog{}, "Opis kvara je obavezan."
	}

	nalog := model.ServisniNalog{
		BrojNaloga:   strings.TrimSpace(r.FormValue("broj_naloga")),
		Uredjaj:      uredjaj,
		SerijskiBroj: strings.TrimSpace(r.FormValue("serijski_broj")),
		OpisKvara:    opisKvara,
		Status:       r.FormValue("status"),
		Napomena:     strings.TrimSpace(r.FormValue("napomena")),
	}

	if nalog.Status == "" {
		nalog.Status = model.StatusPrimljeno
	}

	// opcioni klijent
	if kidStr := r.FormValue("klijent_id"); kidStr != "" {
		if kid, err := strconv.ParseInt(kidStr, 10, 64); err == nil {
			nalog.KlijentID = &kid
		}
	}

	// opcione cene — prazno polje ostaje nil
	nalog.CenaOd = parseOpcionuCenu(r.FormValue("cena_od"))
	nalog.CenaDo = parseOpcionuCenu(r.FormValue("cena_do"))
	nalog.CenaKonacna = parseOpcionuCenu(r.FormValue("cena_konacna"))
	nalog.Avans = parseOpcionuCenu(r.FormValue("avans"))

	// opcioni datum završetka
	if dz := strings.TrimSpace(r.FormValue("datum_zavrsetka")); dz != "" {
		if t, err := time.Parse("2006-01-02", dz); err == nil {
			nalog.DatumZavrsetka = &t
		}
	}

	return nalog, ""
}

// parseOpcionuCenu pretvara string u *float64 — prazno polje ili neispravna vrednost vraća nil
func parseOpcionuCenu(s string) *float64 {
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

// renderujFormuNaloga renderuje HTML šablon forme za unos ili izmenu servisnog naloga
func renderujFormuNaloga(w http.ResponseWriter, podaci PodaciFormeNaloga) {
	tmpl, err := template.ParseFiles(
		"web/templates/teme/podrazumevana/base.html",
		"web/templates/komponente/sidebar.html",
		"web/templates/komponente/topbar.html",
		"web/templates/stranice/servis_forma.html",
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
