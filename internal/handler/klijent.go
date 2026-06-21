package handler

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"ntech/internal/db"
	"ntech/internal/db/sqlite"
	"ntech/internal/model"

	"github.com/go-chi/chi/v5"
)

// PodaciKlijenata su podaci za stranicu sa listom klijenata
type PodaciKlijenata struct {
	model.PodaciStranice
	Klijenti         []model.Klijent
	Pretraga         string
	TipFilter        string
	Sacuvano         bool
	Obrisan          bool
	StranicaBr       int
	UkupnoStranica   int
	UkupnoKlijenata  int
	StranicaPrev     int
	StranicaNext     int
	StranicaQueryUrl string
}

// PodaciFormeKlijenta su podaci za formu novog/izmenjenog klijenta
type PodaciFormeKlijenta struct {
	model.PodaciStranice
	Klijent model.Klijent
	Greska  string
	Izmena  bool
}

// Klijenti renderuje listu svih klijenata sa opcionom pretragom i paginacijom
func (h *Handler) Klijenti(w http.ResponseWriter, r *http.Request) {
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	pretraga := r.URL.Query().Get("pretraga")
	tipFilter := r.URL.Query().Get("tip")
	if tipFilter != "fizicko" && tipFilter != "pravno" {
		tipFilter = ""
	}

	const pageSize = 50
	stranicaBr := 1
	if p := r.URL.Query().Get("stranica"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			stranicaBr = v
		}
	}

	filter := db.KlijentFilter{
		Pretraga: pretraga,
		Tip:      tipFilter,
		Limit:    pageSize,
		Offset:   (stranicaBr - 1) * pageSize,
	}

	klijenti, err := h.KlijentiRepo.ListaFilter(r.Context(), filter)
	if err != nil {
		http.Error(w, "Greška pri učitavanju klijenata", http.StatusInternalServerError)
		return
	}

	ukupno, err := h.KlijentiRepo.PrebrojiPoFilteru(r.Context(), filter)
	if err != nil {
		http.Error(w, "Greška pri učitavanju klijenata", http.StatusInternalServerError)
		return
	}

	ukupnoStranica := (ukupno + pageSize - 1) / pageSize

	queryDelići := ""
	if pretraga != "" {
		queryDelići += "&pretraga=" + pretraga
	}
	if tipFilter != "" {
		queryDelići += "&tip=" + tipFilter
	}

	stranicaPrev := stranicaBr - 1
	if stranicaPrev < 1 {
		stranicaPrev = 1
	}
	stranicaNext := stranicaBr + 1
	if stranicaNext > ukupnoStranica {
		stranicaNext = ukupnoStranica
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "klijenti"
	ps.NaslovStranice = "Klijenti"
	podaci := PodaciKlijenata{
		PodaciStranice:   ps,
		Klijenti:         klijenti,
		Pretraga:         pretraga,
		TipFilter:        tipFilter,
		Sacuvano:         r.URL.Query().Get("sacuvano") == "1",
		Obrisan:          r.URL.Query().Get("obrisan") == "1",
		StranicaBr:       stranicaBr,
		UkupnoStranica:   ukupnoStranica,
		UkupnoKlijenata:  ukupno,
		StranicaPrev:     stranicaPrev,
		StranicaNext:     stranicaNext,
		StranicaQueryUrl: queryDelići,
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

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "klijenti"
	ps.NaslovStranice = "Novi klijent"
	h.renderujFormuKlijenta(w, PodaciFormeKlijenta{
		PodaciStranice: ps,
		Izmena:         false,
	})
}

// SacuvajKlijenta prima POST formu i upisuje novog klijenta u bazu
func (h *Handler) SacuvajKlijenta(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "klijent.dodaj"); !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	klijent, greska := parseFormuKlijenta(r)
	if greska != "" {
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		ps := h.popuniPodaciStranice(r, podesavanja)
		ps.Stranica = "klijenti"
		ps.NaslovStranice = "Novi klijent"
		h.renderujFormuKlijenta(w, PodaciFormeKlijenta{
			PodaciStranice: ps,
			Klijent:        klijent,
			Greska:         greska,
			Izmena:         false,
		})
		return
	}

	if _, err := h.KlijentiRepo.Kreiraj(r.Context(), &klijent); err != nil {
		slog.Error("greška pri čuvanju klijenta", "error", err)
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		ps := h.popuniPodaciStranice(r, podesavanja)
		ps.Stranica = "klijenti"
		ps.NaslovStranice = "Novi klijent"
		h.renderujFormuKlijenta(w, PodaciFormeKlijenta{
			PodaciStranice: ps,
			Klijent:        klijent,
			Greska:         "Došlo je do greške pri čuvanju. Pokušajte ponovo.",
			Izmena:         false,
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

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "klijenti"
	ps.NaslovStranice = "Izmeni klijenta"
	h.renderujFormuKlijenta(w, PodaciFormeKlijenta{
		PodaciStranice: ps,
		Klijent:        *klijent,
		Izmena:         true,
	})
}

// SacuvajIzmenuKlijenta prima POST formu i ažurira postojećeg klijenta u bazi
func (h *Handler) SacuvajIzmenuKlijenta(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "klijent.izmeni"); !ok {
		return
	}
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
		ps := h.popuniPodaciStranice(r, podesavanja)
		ps.Stranica = "klijenti"
		ps.NaslovStranice = "Izmeni klijenta"
		h.renderujFormuKlijenta(w, PodaciFormeKlijenta{
			PodaciStranice: ps,
			Klijent:        klijent,
			Greska:         greska,
			Izmena:         true,
		})
		return
	}

	klijent.ID = id
	if err := h.KlijentiRepo.Izmeni(r.Context(), &klijent); err != nil {
		slog.Error("greška pri čuvanju izmene klijenta", "error", err)
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		ps := h.popuniPodaciStranice(r, podesavanja)
		ps.Stranica = "klijenti"
		ps.NaslovStranice = "Izmeni klijenta"
		h.renderujFormuKlijenta(w, PodaciFormeKlijenta{
			PodaciStranice: ps,
			Klijent:        klijent,
			Greska:         "Došlo je do greške pri čuvanju. Pokušajte ponovo.",
			Izmena:         true,
		})
		return
	}

	http.Redirect(w, r, "/klijenti?sacuvano=1", http.StatusSeeOther)
}

// ObrisiKlijenta prima POST zahtev i briše klijenta po ID-u
func (h *Handler) ObrisiKlijenta(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "klijent.obrisi"); !ok {
		return
	}
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

// validirajJMBG proverava da li uneti JMBG ima tačno 13 cifara. Prazan broj je
// dozvoljen (polje nije obavezno). Vraća srpsku poruku o grešci ili prazan string.
// Broj lične karte se ne proverava jer mu se format razlikuje između izdanja.
func validirajJMBG(broj string) string {
	if broj == "" {
		return ""
	}
	if len(broj) != 13 {
		return "JMBG mora imati tačno 13 cifara."
	}
	for _, c := range broj {
		if c < '0' || c > '9' {
			return "JMBG sme sadržati samo cifre."
		}
	}
	return ""
}

// odrediTipIdentifikacije prepoznaje da li je uneti broj JMBG (13 cifara) ili broj
// lične karte (9 cifara — registarski broj na novoj srpskoj ličnoj karti) i vraća
// odgovarajući tip ("jmbg" ili "licna_karta"). Prazan unos je dozvoljen i vraća
// prazan tip bez greške. Vraća (tip, poruka o grešci).
func odrediTipIdentifikacije(broj string) (string, string) {
	if broj == "" {
		return "", ""
	}
	for _, c := range broj {
		if c < '0' || c > '9' {
			return "", "Identifikacioni broj sme sadržati samo cifre."
		}
	}
	switch len(broj) {
	case 13:
		return "jmbg", ""
	case 9:
		return "licna_karta", ""
	default:
		return "", "Unesite JMBG (13 cifara) ili broj lične karte (9 cifara)."
	}
}

// parseFormuKlijenta čita polja iz HTTP forme, validira ih i vraća model i eventualnu grešku
func parseFormuKlijenta(r *http.Request) (model.Klijent, string) {
	tip := r.FormValue("tip")
	if tip != "fizicko" && tip != "pravno" {
		tip = "fizicko"
	}

	ime := strings.TrimSpace(r.FormValue("ime"))
	nazivFirme := strings.TrimSpace(r.FormValue("naziv_firme"))

	if tip == "fizicko" && ime == "" {
		return model.Klijent{Tip: tip}, "Za fizičko lice obavezno je uneti ime."
	}
	if tip == "pravno" && nazivFirme == "" {
		return model.Klijent{Tip: tip}, "Za pravno lice obavezan je naziv firme."
	}

	email := strings.TrimSpace(r.FormValue("email"))
	if email != "" && !strings.Contains(email, "@") {
		return model.Klijent{Tip: tip}, "Adresa e-pošte nije ispravna."
	}

	// tip identifikacionog broja: 'jmbg' ili 'licna_karta' (podrazumevano jmbg)
	tipIdent := r.FormValue("tip_identifikacije")
	if tipIdent != "jmbg" && tipIdent != "licna_karta" {
		tipIdent = "jmbg"
	}

	jmbg := strings.TrimSpace(r.FormValue("jmbg"))
	if tipIdent == "jmbg" {
		if greska := validirajJMBG(jmbg); greska != "" {
			return model.Klijent{Tip: tip}, greska
		}
	}

	return model.Klijent{
		Tip:               tip,
		Ime:               ime,
		Prezime:           strings.TrimSpace(r.FormValue("prezime")),
		JMBG:              jmbg,
		TipIdentifikacije: tipIdent,
		NazivFirme:        nazivFirme,
		PIB:               strings.TrimSpace(r.FormValue("pib")),
		Telefon:           strings.TrimSpace(r.FormValue("telefon")),
		Email:             email,
		Mesto:             strings.TrimSpace(r.FormValue("mesto")),
		Napomena:          strings.TrimSpace(r.FormValue("napomena")),
	}, ""
}

// renderujFormuKlijenta renderuje HTML šablon forme za unos ili izmenu klijenta
func (h *Handler) renderujFormuKlijenta(w http.ResponseWriter, podaci PodaciFormeKlijenta) {
	h.renderujTemplate(w, "klijent_forma", podaci)
}
