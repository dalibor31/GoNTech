package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"ntech/internal/db/sqlite"
	"ntech/internal/middleware"
	"ntech/internal/model"

	"github.com/go-chi/chi/v5"
)

// PodaciPdvKir su podaci za pregled knjige izdatih računa
type PodaciPdvKir struct {
	model.PodaciStranice
	Zapisi []model.PdvKir
	Sume   model.PdvKirSume
	Od     string // filter perioda (YYYY-MM-DD), prazno = bez granice
	Do     string
}

// PodaciPdvKirForma su podaci za formu unosa zapisa KIR
type PodaciPdvKirForma struct {
	model.PodaciStranice
	Greska   string
	Danas    string          // podrazumevani datum u formi
	Klijenti []model.Klijent // za izbor kupca iz postojećih klijenata
}

// parsiraDatumOpcionalno vraća datum iz YYYY-MM-DD; prazan string daje nulti datum (bez filtera)
func parsiraDatumOpcionalno(s string) time.Time {
	t, err := time.Parse("2006-01-02", strings.TrimSpace(s))
	if err != nil {
		return time.Time{}
	}
	return t
}

// parsiraIznos čita decimalni broj iz forme (prihvata i zarez); prazno/neispravno daje 0
func parsiraIznos(s string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(strings.Replace(s, ",", ".", 1)), 64)
	if err != nil || v < 0 {
		return 0
	}
	return v
}

// PdvKir renderuje pregled knjige izdatih računa sa sumama po stopama
func (h *Handler) PdvKir(w http.ResponseWriter, r *http.Request) {
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
	zapisi, err := h.PdvKirRepo.Lista(r.Context(), parsiraDatumOpcionalno(odStr), parsiraDatumOpcionalno(doStr))
	if err != nil {
		http.Error(w, "Greška pri učitavanju knjige izdatih računa", http.StatusInternalServerError)
		return
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "pdv-kir"
	ps.NaslovStranice = "KIR — knjiga izdatih računa"
	h.renderujTemplate(w, "pdv_kir", PodaciPdvKir{
		PodaciStranice: ps,
		Zapisi:         zapisi,
		Sume:           model.SumirajKir(zapisi),
		Od:             odStr,
		Do:             doStr,
	})
}

// NoviPdvKir prikazuje praznu formu za unos zapisa u KIR
func (h *Handler) NoviPdvKir(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "pdv.dodaj"); !ok {
		return
	}
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}
	// klijenti za izbor kupca; greška se ne prekida (forma radi i sa ručnim unosom)
	klijenti, _ := h.KlijentiRepo.Lista(r.Context(), "")

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "pdv-kir"
	ps.NaslovStranice = "Novi izlazni račun (KIR)"
	h.renderujTemplate(w, "pdv_kir_forma", PodaciPdvKirForma{
		PodaciStranice: ps,
		Danas:          time.Now().Format("2006-01-02"),
		Klijenti:       klijenti,
	})
}

// SacuvajPdvKir prima POST i upisuje novi zapis u KIR
func (h *Handler) SacuvajPdvKir(w http.ResponseWriter, r *http.Request) {
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
	kupacNaziv := strings.TrimSpace(r.FormValue("kupac_naziv"))

	greska := ""
	switch {
	case e1 != nil:
		greska = "Datum prometa je obavezan i mora biti ispravan."
	case e2 != nil:
		greska = "Datum knjiženja je obavezan i mora biti ispravan."
	case brojDokumenta == "":
		greska = "Broj dokumenta je obavezan."
	case kupacNaziv == "":
		greska = "Naziv kupca je obavezan."
	}
	if greska != "" {
		middleware.SetFlash(w, r, h.DB, "greska", greska)
		http.Redirect(w, r, "/pdv/kir/nova", http.StatusSeeOther)
		return
	}

	z := model.PdvKir{
		DatumPrometa:      datumPrometa,
		DatumKnjizenja:    datumKnjizenja,
		BrojDokumenta:     brojDokumenta,
		KupacNaziv:        kupacNaziv,
		KupacPib:          strings.TrimSpace(r.FormValue("kupac_pib")),
		KupacMesto:        strings.TrimSpace(r.FormValue("kupac_mesto")),
		OsnovicaOpsta:     parsiraIznos(r.FormValue("osnovica_opsta")),
		PdvOpsta:          parsiraIznos(r.FormValue("pdv_opsta")),
		OsnovicaPosebna:   parsiraIznos(r.FormValue("osnovica_posebna")),
		PdvPosebna:        parsiraIznos(r.FormValue("pdv_posebna")),
		OslobodenSaPravom: parsiraIznos(r.FormValue("osloboden_sa_pravom")),
		OslobodenBezPrava: parsiraIznos(r.FormValue("osloboden_bez_prava")),
		Napomena:          strings.TrimSpace(r.FormValue("napomena")),
	}
	// ukupna naknada sa PDV — zbir svih osnovica, PDV-a i oslobođenog prometa (računa server)
	z.Ukupno = z.OsnovicaOpsta + z.PdvOpsta + z.OsnovicaPosebna + z.PdvPosebna +
		z.OslobodenSaPravom + z.OslobodenBezPrava

	if _, err := h.PdvKirRepo.Kreiraj(r.Context(), &z); err != nil {
		http.Error(w, "Greška pri čuvanju zapisa", http.StatusInternalServerError)
		return
	}
	middleware.SetFlash(w, r, h.DB, "uspeh", "Izlazni račun je dodat u KIR.")
	http.Redirect(w, r, "/pdv/kir", http.StatusSeeOther)
}

// ObrisiPdvKir briše zapis iz KIR
func (h *Handler) ObrisiPdvKir(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "pdv.obrisi"); !ok {
		return
	}
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID zapisa", http.StatusBadRequest)
		return
	}
	if err := h.PdvKirRepo.Obrisi(r.Context(), id); err != nil {
		http.Error(w, "Greška pri brisanju zapisa", http.StatusInternalServerError)
		return
	}
	middleware.SetFlash(w, r, h.DB, "uspeh", "Zapis je obrisan iz KIR.")
	http.Redirect(w, r, "/pdv/kir", http.StatusSeeOther)
}
