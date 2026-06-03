package handler

import (
	"fmt"
	"net/http"
	"strconv"

	"ntech/internal/db/sqlite"
	"ntech/internal/model"

	"github.com/go-chi/chi/v5"
)

// PodaciFormeArtikla su podaci za formu novog/izmenjenog artikla
type PodaciFormeArtikla struct {
	model.PodaciStranice
	Artikal         model.Artikal
	Kategorije      []model.Kategorija
	KategorijaIDStr string
	Greska          string
	Izmena          bool
}

// NoviArtikal prikazuje formu za unos novog artikla
func (h *Handler) NoviArtikal(w http.ResponseWriter, r *http.Request) {
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	kategorije, err := h.KategorijeRepo.Lista(r.Context())
	if err != nil {
		http.Error(w, "Greška pri učitavanju kategorija", http.StatusInternalServerError)
		return
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "magacin"
	ps.NaslovStranice = "Novi artikal"
	h.renderujFormuArtikla(w, PodaciFormeArtikla{
		PodaciStranice: ps,
		Kategorije:     kategorije,
		Izmena:         false,
	})
}

// SacuvajArtikal prima POST formu i čuva novi artikal
func (h *Handler) SacuvajArtikal(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	artikal, greska := parseFormuArtikla(r)
	if greska != "" {
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		kategorije, _ := h.KategorijeRepo.Lista(r.Context())
		katIDStr := ""
		if artikal.KategorijaID != nil {
			katIDStr = strconv.FormatInt(*artikal.KategorijaID, 10)
		}
		ps := h.popuniPodaciStranice(r, podesavanja)
		ps.Stranica = "magacin"
		ps.NaslovStranice = "Novi artikal"
		h.renderujFormuArtikla(w, PodaciFormeArtikla{
			PodaciStranice:  ps,
			Artikal:         artikal,
			Kategorije:      kategorije,
			KategorijaIDStr: katIDStr,
			Greska:          greska,
			Izmena:          false,
		})
		return
	}

	id, err := h.Artikli.Kreiraj(r.Context(), &artikal)
	if err != nil {
		http.Error(w, "Greška pri čuvanju artikla", http.StatusInternalServerError)
		return
	}

	// fetch zahtev (iz modala) dobija JSON sa ID-em i nazivom novog artikla
	if r.Header.Get("X-Requested-With") == "fetch" {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":%d,"naziv":%q}`, id, artikal.Naziv)
		return
	}

	http.Redirect(w, r, "/magacin?sacuvano=1", http.StatusSeeOther)
}

// IzmeniArtikal prikazuje formu za izmenu postojećeg artikla
func (h *Handler) IzmeniArtikal(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Neispravan ID artikla", http.StatusBadRequest)
		return
	}

	artikal, err := h.Artikli.DohvatiID(r.Context(), id)
	if err != nil {
		http.Error(w, "Artikal nije pronađen", http.StatusNotFound)
		return
	}

	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	kategorije, err := h.KategorijeRepo.Lista(r.Context())
	if err != nil {
		http.Error(w, "Greška pri učitavanju kategorija", http.StatusInternalServerError)
		return
	}

	katIDStr := ""
	if artikal.KategorijaID != nil {
		katIDStr = strconv.FormatInt(*artikal.KategorijaID, 10)
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "magacin"
	ps.NaslovStranice = "Izmeni artikal"
	h.renderujFormuArtikla(w, PodaciFormeArtikla{
		PodaciStranice:  ps,
		Artikal:         *artikal,
		Kategorije:      kategorije,
		KategorijaIDStr: katIDStr,
		Izmena:          true,
	})
}

// SacuvajIzmenuArtikla prima POST formu i čuva izmenu artikla
func (h *Handler) SacuvajIzmenuArtikla(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Neispravan ID artikla", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	artikal, greska := parseFormuArtikla(r)
	if greska != "" {
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		kategorije, _ := h.KategorijeRepo.Lista(r.Context())
		artikal.ID = id
		katIDStr := ""
		if artikal.KategorijaID != nil {
			katIDStr = strconv.FormatInt(*artikal.KategorijaID, 10)
		}
		ps := h.popuniPodaciStranice(r, podesavanja)
		ps.Stranica = "magacin"
		ps.NaslovStranice = "Izmeni artikal"
		h.renderujFormuArtikla(w, PodaciFormeArtikla{
			PodaciStranice:  ps,
			Artikal:         artikal,
			Kategorije:      kategorije,
			KategorijaIDStr: katIDStr,
			Greska:          greska,
			Izmena:          true,
		})
		return
	}

	artikal.ID = id
	if err := h.Artikli.Izmeni(r.Context(), &artikal); err != nil {
		http.Error(w, "Greška pri čuvanju izmene", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/magacin?sacuvano=1", http.StatusSeeOther)
}

// parseFormuArtikla čita polja iz forme i vraća artikal i eventualnu grešku
func parseFormuArtikla(r *http.Request) (model.Artikal, string) {
	naziv := r.FormValue("naziv")
	if naziv == "" {
		return model.Artikal{}, "Naziv artikla je obavezan."
	}

	var artikal model.Artikal
	artikal.Naziv = naziv
	artikal.Opis = r.FormValue("opis")
	artikal.Lokacija = r.FormValue("lokacija")
	artikal.Napomena = r.FormValue("napomena")

	if k := r.FormValue("kolicina"); k != "" {
		v, err := strconv.Atoi(k)
		if err != nil || v < 0 {
			return artikal, "Količina mora biti pozitivan broj."
		}
		artikal.Kolicina = v
	}

	if k := r.FormValue("kolicina_min"); k != "" {
		v, err := strconv.Atoi(k)
		if err != nil || v < 0 {
			return artikal, "Minimalna količina mora biti pozitivan broj."
		}
		artikal.KolicinMin = v
	}

	if c := r.FormValue("prodajna_cena"); c != "" {
		v, err := strconv.ParseFloat(c, 64)
		if err != nil || v < 0 {
			return artikal, "Prodajna cena mora biti pozitivan broj."
		}
		artikal.ProdajnaCena = v
	}

	if katID := r.FormValue("kategorija_id"); katID != "" {
		id, err := strconv.ParseInt(katID, 10, 64)
		if err == nil {
			artikal.KategorijaID = &id
		}
	}

	return artikal, ""
}

// renderujFormuArtikla renderuje HTML formu za artikal
func (h *Handler) renderujFormuArtikla(w http.ResponseWriter, podaci PodaciFormeArtikla) {
	h.renderujTemplate(w, "magacin_forma", podaci)
}
