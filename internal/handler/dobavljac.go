package handler

import (
	"html/template"
	"log"
	"net/http"
	"strings"

	"ntech/internal/db/sqlite"
	"ntech/internal/model"

	"github.com/go-chi/chi/v5"
)

// PodaciDobavljaca su podaci za stranicu sa listom dobavljača
type PodaciDobavljaca struct {
	model.PodaciStranice
	Dobavljaci []model.Dobavljac
	Pretraga   string
	Sacuvano   bool
	Obrisan    bool
}

// PodaciFormeDobavljaca su podaci za formu novog/izmenjenog dobavljača
type PodaciFormeDobavljaca struct {
	model.PodaciStranice
	Dobavljac model.Dobavljac
	Greska    string
	Izmena    bool
}

// Dobavljaci renderuje listu svih dobavljača
func (h *Handler) Dobavljaci(w http.ResponseWriter, r *http.Request) {
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	pretraga := r.URL.Query().Get("pretraga")

	dobavljaci, err := h.DobavljaciRepo.Lista(r.Context(), pretraga)
	if err != nil {
		http.Error(w, "Greška pri učitavanju dobavljača", http.StatusInternalServerError)
		return
	}

	podaci := PodaciDobavljaca{
		PodaciStranice: model.PodaciStranice{
			Stranica:       "dobavljaci",
			NaslovStranice: "Dobavljači",
			Tema:           podesavanja["tema"],
			NazivFirme:     podesavanja["naziv_firme"],
			Podnazlov:      podesavanja["podnazlov"],
			LogoTip:        podesavanja["logo_tip"],
			LogoPutanja:    podesavanja["logo_putanja"],
			Korisnik:       "Admin",
		},
		Dobavljaci: dobavljaci,
		Pretraga:   pretraga,
		Sacuvano:   r.URL.Query().Get("sacuvano") == "1",
		Obrisan:    r.URL.Query().Get("obrisan") == "1",
	}

	tmpl, err := template.ParseFiles(
		"web/templates/teme/podrazumevana/base.html",
		"web/templates/komponente/sidebar.html",
		"web/templates/komponente/topbar.html",
		"web/templates/stranice/dobavljaci.html",
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

// NoviDobavljac prikazuje praznu formu za unos novog dobavljača
func (h *Handler) NoviDobavljac(w http.ResponseWriter, r *http.Request) {
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	renderujFormuDobavljaca(w, PodaciFormeDobavljaca{
		PodaciStranice: model.PodaciStranice{
			Stranica:       "dobavljaci",
			NaslovStranice: "Novi dobavljač",
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

// SacuvajDobavljaca prima POST formu i upisuje novog dobavljača u bazu
func (h *Handler) SacuvajDobavljaca(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	dobavljac, greska := parseFormuDobavljaca(r)
	if greska != "" {
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		renderujFormuDobavljaca(w, PodaciFormeDobavljaca{
			PodaciStranice: model.PodaciStranice{
				Stranica:       "dobavljaci",
				NaslovStranice: "Novi dobavljač",
				Tema:           podesavanja["tema"],
				NazivFirme:     podesavanja["naziv_firme"],
				Podnazlov:      podesavanja["podnazlov"],
				LogoTip:        podesavanja["logo_tip"],
				LogoPutanja:    podesavanja["logo_putanja"],
				Korisnik:       "Admin",
			},
			Dobavljac: dobavljac,
			Greska:    greska,
			Izmena:    false,
		})
		return
	}

	if _, err := h.DobavljaciRepo.Kreiraj(r.Context(), &dobavljac); err != nil {
		http.Error(w, "Greška pri čuvanju dobavljača", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/dobavljaci?sacuvano=1", http.StatusSeeOther)
}

// IzmeniDobavljaca učitava dobavljača po ID-u i prikazuje popunjenu formu za izmenu
func (h *Handler) IzmeniDobavljaca(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID dobavljača", http.StatusBadRequest)
		return
	}

	dobavljac, err := h.DobavljaciRepo.DohvatiID(r.Context(), id)
	if err != nil {
		http.Error(w, "Dobavljač nije pronađen", http.StatusNotFound)
		return
	}

	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	renderujFormuDobavljaca(w, PodaciFormeDobavljaca{
		PodaciStranice: model.PodaciStranice{
			Stranica:       "dobavljaci",
			NaslovStranice: "Izmeni dobavljača",
			Tema:           podesavanja["tema"],
			NazivFirme:     podesavanja["naziv_firme"],
			Podnazlov:      podesavanja["podnazlov"],
			LogoTip:        podesavanja["logo_tip"],
			LogoPutanja:    podesavanja["logo_putanja"],
			Korisnik:       "Admin",
		},
		Dobavljac: *dobavljac,
		Izmena:    true,
	})
}

// SacuvajIzmeneDobavljaca prima POST formu i ažurira postojećeg dobavljača u bazi
func (h *Handler) SacuvajIzmeneDobavljaca(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID dobavljača", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	dobavljac, greska := parseFormuDobavljaca(r)
	if greska != "" {
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		dobavljac.ID = id
		renderujFormuDobavljaca(w, PodaciFormeDobavljaca{
			PodaciStranice: model.PodaciStranice{
				Stranica:       "dobavljaci",
				NaslovStranice: "Izmeni dobavljača",
				Tema:           podesavanja["tema"],
				NazivFirme:     podesavanja["naziv_firme"],
				Podnazlov:      podesavanja["podnazlov"],
				LogoTip:        podesavanja["logo_tip"],
				LogoPutanja:    podesavanja["logo_putanja"],
				Korisnik:       "Admin",
			},
			Dobavljac: dobavljac,
			Greska:    greska,
			Izmena:    true,
		})
		return
	}

	dobavljac.ID = id
	if err := h.DobavljaciRepo.Izmeni(r.Context(), &dobavljac); err != nil {
		http.Error(w, "Greška pri čuvanju izmene", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/dobavljaci?sacuvano=1", http.StatusSeeOther)
}

// ObrisiDobavljaca prima POST zahtev i briše dobavljača po ID-u
func (h *Handler) ObrisiDobavljaca(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID dobavljača", http.StatusBadRequest)
		return
	}

	if err := h.DobavljaciRepo.Obrisi(r.Context(), id); err != nil {
		http.Error(w, "Greška pri brisanju dobavljača", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/dobavljaci?obrisan=1", http.StatusSeeOther)
}

// parseFormuDobavljaca čita polja iz HTTP forme, validira ih i vraća model i eventualnu grešku
func parseFormuDobavljaca(r *http.Request) (model.Dobavljac, string) {
	naziv := strings.TrimSpace(r.FormValue("naziv"))
	if naziv == "" {
		return model.Dobavljac{}, "Naziv dobavljača je obavezan."
	}

	email := strings.TrimSpace(r.FormValue("email"))
	if email != "" && !strings.Contains(email, "@") {
		return model.Dobavljac{}, "Adresa e-pošte nije ispravna."
	}

	return model.Dobavljac{
		Naziv:        naziv,
		KontaktOsoba: strings.TrimSpace(r.FormValue("kontakt_osoba")),
		Telefon:      strings.TrimSpace(r.FormValue("telefon")),
		Email:        email,
		Napomena:     strings.TrimSpace(r.FormValue("napomena")),
	}, ""
}

// renderujFormuDobavljaca renderuje HTML šablon forme za unos ili izmenu dobavljača
func renderujFormuDobavljaca(w http.ResponseWriter, podaci PodaciFormeDobavljaca) {
	tmpl, err := template.ParseFiles(
		"web/templates/teme/podrazumevana/base.html",
		"web/templates/komponente/sidebar.html",
		"web/templates/komponente/topbar.html",
		"web/templates/stranice/dobavljac_forma.html",
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
