package handler

import (
	"fmt"
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

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "dobavljaci"
	ps.NaslovStranice = "Dobavljači"
	podaci := PodaciDobavljaca{
		PodaciStranice: ps,
		Dobavljaci:     dobavljaci,
		Pretraga:       pretraga,
		Sacuvano:       r.URL.Query().Get("sacuvano") == "1",
		Obrisan:        r.URL.Query().Get("obrisan") == "1",
	}

	h.renderujTemplate(w, "dobavljaci", podaci)
}

// NoviDobavljac prikazuje praznu formu za unos novog dobavljača
func (h *Handler) NoviDobavljac(w http.ResponseWriter, r *http.Request) {
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "dobavljaci"
	ps.NaslovStranice = "Novi dobavljač"
	h.renderujFormuDobavljaca(w, PodaciFormeDobavljaca{
		PodaciStranice: ps,
		Izmena:         false,
	})
}

// SacuvajDobavljaca prima POST formu i upisuje novog dobavljača u bazu
func (h *Handler) SacuvajDobavljaca(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "dobavljac.dodaj"); !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	dobavljac, greska := parseFormuDobavljaca(r)
	jeAjax := r.Header.Get("X-Requested-With") == "fetch"
	if greska != "" {
		// fetch zahtev (iz modala u nabavci) dobija JSON grešku
		if jeAjax {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"greska":%q}`, greska)
			return
		}
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		ps := h.popuniPodaciStranice(r, podesavanja)
		ps.Stranica = "dobavljaci"
		ps.NaslovStranice = "Novi dobavljač"
		h.renderujFormuDobavljaca(w, PodaciFormeDobavljaca{
			PodaciStranice: ps,
			Dobavljac:      dobavljac,
			Greska:         greska,
			Izmena:         false,
		})
		return
	}

	id, err := h.DobavljaciRepo.Kreiraj(r.Context(), &dobavljac)
	if err != nil {
		http.Error(w, "Greška pri čuvanju dobavljača", http.StatusInternalServerError)
		return
	}

	// fetch zahtev (iz modala) dobija JSON sa ID-em i nazivom novog dobavljača
	if jeAjax {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":%d,"naziv":%q}`, id, dobavljac.Naziv)
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

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "dobavljaci"
	ps.NaslovStranice = "Izmeni dobavljača"
	h.renderujFormuDobavljaca(w, PodaciFormeDobavljaca{
		PodaciStranice: ps,
		Dobavljac:      *dobavljac,
		Izmena:         true,
	})
}

// SacuvajIzmeneDobavljaca prima POST formu i ažurira postojećeg dobavljača u bazi
func (h *Handler) SacuvajIzmeneDobavljaca(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "dobavljac.izmeni"); !ok {
		return
	}
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
		ps := h.popuniPodaciStranice(r, podesavanja)
		ps.Stranica = "dobavljaci"
		ps.NaslovStranice = "Izmeni dobavljača"
		h.renderujFormuDobavljaca(w, PodaciFormeDobavljaca{
			PodaciStranice: ps,
			Dobavljac:      dobavljac,
			Greska:         greska,
			Izmena:         true,
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
	if _, ok := h.zahtevajDozvolu(w, r, "dobavljac.obrisi"); !ok {
		return
	}
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
		PIB:          strings.TrimSpace(r.FormValue("pib")),
		Mesto:        strings.TrimSpace(r.FormValue("mesto")),
		Napomena:     strings.TrimSpace(r.FormValue("napomena")),
	}, ""
}

// renderujFormuDobavljaca renderuje HTML šablon forme za unos ili izmenu dobavljača
func (h *Handler) renderujFormuDobavljaca(w http.ResponseWriter, podaci PodaciFormeDobavljaca) {
	h.renderujTemplate(w, "dobavljac_forma", podaci)
}
