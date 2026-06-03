package handler

import (
	"log"
	"net/http"
	"strings"

	"ntech/internal/db/sqlite"
	"ntech/internal/model"

	"github.com/go-chi/chi/v5"
)

// PodaciKlijenata su podaci za stranicu sa listom klijenata
type PodaciKlijenata struct {
	model.PodaciStranice
	Klijenti []model.Klijent
	Pretraga string
	Sacuvano bool
	Obrisan  bool
}

// PodaciFormeKlijenta su podaci za formu novog/izmenjenog klijenta
type PodaciFormeKlijenta struct {
	model.PodaciStranice
	Klijent model.Klijent
	Greska  string
	Izmena  bool
}

// Klijenti renderuje listu svih klijenata sa opcionom pretragom
func (h *Handler) Klijenti(w http.ResponseWriter, r *http.Request) {
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	pretraga := r.URL.Query().Get("pretraga")

	klijenti, err := h.KlijentiRepo.Lista(r.Context(), pretraga)
	if err != nil {
		http.Error(w, "Greška pri učitavanju klijenata", http.StatusInternalServerError)
		return
	}

	podaci := PodaciKlijenata{
		PodaciStranice: model.PodaciStranice{
			Stranica:       "klijenti",
			NaslovStranice: "Klijenti",
			Tema:           podesavanja["tema"],
			NazivFirme:     podesavanja["naziv_firme"],
			Podnazlov:      podesavanja["podnazlov"],
			LogoTip:        podesavanja["logo_tip"],
			LogoPutanja:    podesavanja["logo_putanja"],
			Korisnik:       "Admin",
		},
		Klijenti: klijenti,
		Pretraga: pretraga,
		Sacuvano: r.URL.Query().Get("sacuvano") == "1",
		Obrisan:  r.URL.Query().Get("obrisan") == "1",
	}

	h.renderujTemplate(w, "klijenti", podaci)
}

// NoviKlijent prikazuje praznu formu za unos novog klijenta
func (h *Handler) NoviKlijent(w http.ResponseWriter, r *http.Request) {
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	h.renderujFormuKlijenta(w, PodaciFormeKlijenta{
		PodaciStranice: model.PodaciStranice{
			Stranica:       "klijenti",
			NaslovStranice: "Novi klijent",
			Tema:           podesavanja["tema"],
			NazivFirme:     podesavanja["naziv_firme"],
			Podnazlov:      podesavanja["podnazlov"],
			LogoTip:        podesavanja["logo_tip"],
			LogoPutanja:    podesavanja["logo_putanja"],
			Korisnik:       "Admin",
		},
		Izmena: false,
	})
}

// SacuvajKlijenta prima POST formu i upisuje novog klijenta u bazu
func (h *Handler) SacuvajKlijenta(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	klijent, greska := parseFormuKlijenta(r)
	if greska != "" {
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		h.renderujFormuKlijenta(w, PodaciFormeKlijenta{
			PodaciStranice: model.PodaciStranice{
				Stranica:       "klijenti",
				NaslovStranice: "Novi klijent",
				Tema:           podesavanja["tema"],
				NazivFirme:     podesavanja["naziv_firme"],
				Podnazlov:      podesavanja["podnazlov"],
				LogoTip:        podesavanja["logo_tip"],
				LogoPutanja:    podesavanja["logo_putanja"],
				Korisnik:       "Admin",
			},
			Klijent: klijent,
			Greska:  greska,
			Izmena:  false,
		})
		return
	}

	if _, err := h.KlijentiRepo.Kreiraj(r.Context(), &klijent); err != nil {
		log.Printf("greška pri čuvanju klijenta: %v", err)
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		h.renderujFormuKlijenta(w, PodaciFormeKlijenta{
			PodaciStranice: model.PodaciStranice{
				Stranica:       "klijenti",
				NaslovStranice: "Novi klijent",
				Tema:           podesavanja["tema"],
				NazivFirme:     podesavanja["naziv_firme"],
				Podnazlov:      podesavanja["podnazlov"],
				LogoTip:        podesavanja["logo_tip"],
				LogoPutanja:    podesavanja["logo_putanja"],
				Korisnik:       "Admin",
			},
			Klijent: klijent,
			Greska:  "Došlo je do greške pri čuvanju. Pokušajte ponovo.",
			Izmena:  false,
		})
		return
	}

	http.Redirect(w, r, "/klijenti?sacuvano=1", http.StatusSeeOther)
}

// IzmeniKlijenta učitava klijenta po ID-u i prikazuje popunjenu formu za izmenu
func (h *Handler) IzmeniKlijenta(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID klijenta", http.StatusBadRequest)
		return
	}

	klijent, err := h.KlijentiRepo.DohvatiID(r.Context(), id)
	if err != nil {
		http.Error(w, "Klijent nije pronađen", http.StatusNotFound)
		return
	}

	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	h.renderujFormuKlijenta(w, PodaciFormeKlijenta{
		PodaciStranice: model.PodaciStranice{
			Stranica:       "klijenti",
			NaslovStranice: "Izmeni klijenta",
			Tema:           podesavanja["tema"],
			NazivFirme:     podesavanja["naziv_firme"],
			Podnazlov:      podesavanja["podnazlov"],
			LogoTip:        podesavanja["logo_tip"],
			LogoPutanja:    podesavanja["logo_putanja"],
			Korisnik:       "Admin",
		},
		Klijent: *klijent,
		Izmena:  true,
	})
}

// SacuvajIzmenuKlijenta prima POST formu i ažurira postojećeg klijenta u bazi
func (h *Handler) SacuvajIzmenuKlijenta(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID klijenta", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	klijent, greska := parseFormuKlijenta(r)
	if greska != "" {
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		klijent.ID = id
		h.renderujFormuKlijenta(w, PodaciFormeKlijenta{
			PodaciStranice: model.PodaciStranice{
				Stranica:       "klijenti",
				NaslovStranice: "Izmeni klijenta",
				Tema:           podesavanja["tema"],
				NazivFirme:     podesavanja["naziv_firme"],
				Podnazlov:      podesavanja["podnazlov"],
				LogoTip:        podesavanja["logo_tip"],
				LogoPutanja:    podesavanja["logo_putanja"],
				Korisnik:       "Admin",
			},
			Klijent: klijent,
			Greska:  greska,
			Izmena:  true,
		})
		return
	}

	klijent.ID = id
	if err := h.KlijentiRepo.Izmeni(r.Context(), &klijent); err != nil {
		log.Printf("greška pri čuvanju izmene klijenta: %v", err)
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		h.renderujFormuKlijenta(w, PodaciFormeKlijenta{
			PodaciStranice: model.PodaciStranice{
				Stranica:       "klijenti",
				NaslovStranice: "Izmeni klijenta",
				Tema:           podesavanja["tema"],
				NazivFirme:     podesavanja["naziv_firme"],
				Podnazlov:      podesavanja["podnazlov"],
				LogoTip:        podesavanja["logo_tip"],
				LogoPutanja:    podesavanja["logo_putanja"],
				Korisnik:       "Admin",
			},
			Klijent: klijent,
			Greska:  "Došlo je do greške pri čuvanju. Pokušajte ponovo.",
			Izmena:  true,
		})
		return
	}

	http.Redirect(w, r, "/klijenti?sacuvano=1", http.StatusSeeOther)
}

// ObrisiKlijenta prima POST zahtev i briše klijenta po ID-u
func (h *Handler) ObrisiKlijenta(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID klijenta", http.StatusBadRequest)
		return
	}

	if err := h.KlijentiRepo.Obrisi(r.Context(), id); err != nil {
		http.Error(w, "Greška pri brisanju klijenta", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/klijenti?obrisan=1", http.StatusSeeOther)
}

// parseFormuKlijenta čita polja iz HTTP forme, validira ih i vraća model i eventualnu grešku
func parseFormuKlijenta(r *http.Request) (model.Klijent, string) {
	ime := strings.TrimSpace(r.FormValue("ime"))
	nazivFirme := strings.TrimSpace(r.FormValue("naziv_firme"))

	if ime == "" && nazivFirme == "" {
		return model.Klijent{}, "Mora biti uneto ime i prezime ili naziv firme."
	}

	email := strings.TrimSpace(r.FormValue("email"))
	if email != "" && !strings.Contains(email, "@") {
		return model.Klijent{}, "Adresa e-pošte nije ispravna."
	}

	return model.Klijent{
		Ime:        ime,
		Prezime:    strings.TrimSpace(r.FormValue("prezime")),
		NazivFirme: nazivFirme,
		PIB:        strings.TrimSpace(r.FormValue("pib")),
		Telefon:    strings.TrimSpace(r.FormValue("telefon")),
		Email:      email,
		Napomena:   strings.TrimSpace(r.FormValue("napomena")),
	}, ""
}

// renderujFormuKlijenta renderuje HTML šablon forme za unos ili izmenu klijenta
func (h *Handler) renderujFormuKlijenta(w http.ResponseWriter, podaci PodaciFormeKlijenta) {
	h.renderujTemplate(w, "klijent_forma", podaci)
}
