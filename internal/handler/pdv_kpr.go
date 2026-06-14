package handler

import (
	"net/http"
	"strings"
	"time"

	"ntech/internal/db/sqlite"
	"ntech/internal/middleware"
	"ntech/internal/model"

	"github.com/go-chi/chi/v5"
)

// PodaciPdvKpr su podaci za pregled knjige primljenih računa
type PodaciPdvKpr struct {
	model.PodaciStranice
	Zapisi []model.PdvKpr
	Sume   model.PdvKprSume
	Od     string
	Do     string
}

// PodaciPdvKprForma su podaci za formu unosa zapisa KPR
type PodaciPdvKprForma struct {
	model.PodaciStranice
	Greska     string
	Danas      string
	Dobavljaci []model.Dobavljac // za izbor dobavljača iz postojećih
}

// PdvKpr renderuje pregled knjige primljenih računa sa sumama po stopama
func (h *Handler) PdvKpr(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "pdv.pregled"); !ok {
		return
	}
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	odStr := r.URL.Query().Get("od")
	doStr := r.URL.Query().Get("do")
	zapisi, err := h.PdvKprRepo.Lista(r.Context(), parsiraDatumOpcionalno(odStr), parsiraDatumOpcionalno(doStr))
	if err != nil {
		http.Error(w, "Greška pri učitavanju knjige primljenih računa", http.StatusInternalServerError)
		return
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "pdv-kpr"
	ps.NaslovStranice = "KPR — knjiga primljenih računa"
	h.renderujTemplate(w, "pdv_kpr", PodaciPdvKpr{
		PodaciStranice: ps,
		Zapisi:         zapisi,
		Sume:           model.SumirajKpr(zapisi),
		Od:             odStr,
		Do:             doStr,
	})
}

// NoviPdvKpr prikazuje praznu formu za unos zapisa u KPR
func (h *Handler) NoviPdvKpr(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "pdv.dodaj"); !ok {
		return
	}
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}
	dobavljaci, _ := h.DobavljaciRepo.Lista(r.Context(), "")

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "pdv-kpr"
	ps.NaslovStranice = "Novi ulazni račun (KPR)"
	h.renderujTemplate(w, "pdv_kpr_forma", PodaciPdvKprForma{
		PodaciStranice: ps,
		Danas:          time.Now().Format("2006-01-02"),
		Dobavljaci:     dobavljaci,
	})
}

// SacuvajPdvKpr prima POST i upisuje novi zapis u KPR
func (h *Handler) SacuvajPdvKpr(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "pdv.dodaj"); !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	datumPrometa, e1 := time.Parse("2006-01-02", strings.TrimSpace(r.FormValue("datum_prometa")))
	datumKnjizenja, e2 := time.Parse("2006-01-02", strings.TrimSpace(r.FormValue("datum_knjizenja")))
	brojDokumenta := strings.TrimSpace(r.FormValue("broj_dokumenta"))
	dobavljacNaziv := strings.TrimSpace(r.FormValue("dobavljac_naziv"))

	greska := ""
	switch {
	case e1 != nil:
		greska = "Datum prometa je obavezan i mora biti ispravan."
	case e2 != nil:
		greska = "Datum knjiženja je obavezan i mora biti ispravan."
	case brojDokumenta == "":
		greska = "Broj dokumenta je obavezan."
	case dobavljacNaziv == "":
		greska = "Naziv dobavljača je obavezan."
	}
	if greska != "" {
		middleware.SetFlash(w, r, h.DB, "greska", greska)
		http.Redirect(w, r, "/pdv/kpr/nova", http.StatusSeeOther)
		return
	}

	z := model.PdvKpr{
		DatumPrometa:     datumPrometa,
		DatumKnjizenja:   datumKnjizenja,
		BrojDokumenta:    brojDokumenta,
		DobavljacNaziv:   dobavljacNaziv,
		DobavljacPib:     strings.TrimSpace(r.FormValue("dobavljac_pib")),
		DobavljacMesto:   strings.TrimSpace(r.FormValue("dobavljac_mesto")),
		OsnovicaOpsta:    parsiraIznos(r.FormValue("osnovica_opsta")),
		PdvOpsta:         parsiraIznos(r.FormValue("pdv_opsta")),
		OsnovicaPosebna:  parsiraIznos(r.FormValue("osnovica_posebna")),
		PdvPosebna:       parsiraIznos(r.FormValue("pdv_posebna")),
		PdvBezOdbitka:    parsiraIznos(r.FormValue("pdv_bez_odbitka")),
		OslobodenNabavka: parsiraIznos(r.FormValue("osloboden_nabavka")),
		Napomena:         strings.TrimSpace(r.FormValue("napomena")),
	}
	// datum plaćanja je opcionalan
	if dp := parsiraDatumOpcionalno(r.FormValue("datum_placanja")); !dp.IsZero() {
		z.DatumPlacanja = &dp
	}
	// ukupna vrednost računa — zbir osnovica, PDV-a i oslobođene nabavke (računa server)
	z.Ukupno = z.OsnovicaOpsta + z.PdvOpsta + z.OsnovicaPosebna + z.PdvPosebna +
		z.PdvBezOdbitka + z.OslobodenNabavka

	if _, err := h.PdvKprRepo.Kreiraj(r.Context(), &z); err != nil {
		http.Error(w, "Greška pri čuvanju zapisa", http.StatusInternalServerError)
		return
	}
	middleware.SetFlash(w, r, h.DB, "uspeh", "Ulazni račun je dodat u KPR.")
	http.Redirect(w, r, "/pdv/kpr", http.StatusSeeOther)
}

// ObrisiPdvKpr briše zapis iz KPR
func (h *Handler) ObrisiPdvKpr(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "pdv.obrisi"); !ok {
		return
	}
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID zapisa", http.StatusBadRequest)
		return
	}
	if err := h.PdvKprRepo.Obrisi(r.Context(), id); err != nil {
		http.Error(w, "Greška pri brisanju zapisa", http.StatusInternalServerError)
		return
	}
	middleware.SetFlash(w, r, h.DB, "uspeh", "Zapis je obrisan iz KPR.")
	http.Redirect(w, r, "/pdv/kpr", http.StatusSeeOther)
}
