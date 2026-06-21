package handler

import (
	"context"
	"encoding/base64"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	appdbPkg "ntech/internal/db"
	"ntech/internal/db/sqlite"
	"ntech/internal/middleware"
	"ntech/internal/model"

	"github.com/go-chi/chi/v5"
	qrcode "github.com/skip2/go-qrcode"
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
	Tehnicari  []model.Korisnik
	SviStatusi []string
	Greska     string
	Izmena     bool
}

// PodaciDetaljiNaloga su podaci za pregled jednog servisnog naloga
type PodaciDetaljiNaloga struct {
	model.PodaciStranice
	Nalog          model.ServisniNalog
	KlijentNaziv   string
	TehnicarNaziv  string
	ServisniDelovi []model.ServisniDeoSaArtiklom
	Artikli        []model.ArtikalSaKategorijom
	Sacuvano       bool
	UkupnoDelovi   float64
	UkupnoSve      float64
	PreostaloSve   float64
	SviStatusi     []string
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

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "servis"
	ps.NaslovStranice = "Servis"
	podaci := PodaciServisa{
		PodaciStranice: ps,
		Nalozi:         nalozi,
		Pretraga:       pretraga,
		FilterStatus:   filterStatus,
		SviStatusi:     model.SviStatusi,
		Sacuvano:       r.URL.Query().Get("sacuvano") == "1",
		Obrisan:        r.URL.Query().Get("obrisan") == "1",
	}

	h.renderujTemplate(w, "servis", podaci)
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

	tehnicari, err := h.KorisniciRepo.Lista(r.Context())
	if err != nil {
		http.Error(w, "Greška pri učitavanju tehničara", http.StatusInternalServerError)
		return
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "servis"
	ps.NaslovStranice = "Novi nalog"
	noviNalog := model.ServisniNalog{
		BrojNaloga:   brojNaloga,
		Status:       model.StatusPrimljeno,
		DatumPrijema: time.Now(),
	}
	noviNalog.GarancijaDo = defaultGarancija(noviNalog.DatumPrijema, podesavanja)
	h.renderujFormuNaloga(w, PodaciFormeNaloga{
		PodaciStranice: ps,
		Nalog:          noviNalog,
		Klijenti:       klijenti,
		Tehnicari:      tehnicari,
		SviStatusi:     model.SviStatusi,
		Izmena:         false,
	})
}

// SacuvajNalog prima POST formu i upisuje novi servisni nalog u bazu
func (h *Handler) SacuvajNalog(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "servis.dodaj"); !ok {
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
		tehnicari, _ := h.KorisniciRepo.Lista(r.Context())
		ps := h.popuniPodaciStranice(r, podesavanja)
		ps.Stranica = "servis"
		ps.NaslovStranice = "Novi nalog"
		h.renderujFormuNaloga(w, PodaciFormeNaloga{
			PodaciStranice: ps,
			Nalog:          nalog,
			Klijenti:       klijenti,
			Tehnicari:      tehnicari,
			SviStatusi:     model.SviStatusi,
			Greska:         greska,
			Izmena:         false,
		})
		return
	}

	// ako nije izabran postojeći klijent, eventualno kreiraj novog iz polja forme
	if greska := h.mozdaKreirajKlijenta(r.Context(), r, &nalog); greska != "" {
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		klijenti, _ := h.KlijentiRepo.Lista(r.Context(), "")
		tehnicari, _ := h.KorisniciRepo.Lista(r.Context())
		ps := h.popuniPodaciStranice(r, podesavanja)
		ps.Stranica = "servis"
		ps.NaslovStranice = "Novi nalog"
		h.renderujFormuNaloga(w, PodaciFormeNaloga{
			PodaciStranice: ps,
			Nalog:          nalog,
			Klijenti:       klijenti,
			Tehnicari:      tehnicari,
			SviStatusi:     model.SviStatusi,
			Greska:         greska,
			Izmena:         false,
		})
		return
	}

	id, err := h.ServisRepo.Kreiraj(r.Context(), &nalog)
	if err != nil {
		slog.Error("greška pri čuvanju naloga", "error", err)
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		klijenti, _ := h.KlijentiRepo.Lista(r.Context(), "")
		tehnicari, _ := h.KorisniciRepo.Lista(r.Context())
		ps := h.popuniPodaciStranice(r, podesavanja)
		ps.Stranica = "servis"
		ps.NaslovStranice = "Novi nalog"
		h.renderujFormuNaloga(w, PodaciFormeNaloga{
			PodaciStranice: ps,
			Nalog:          nalog,
			Klijenti:       klijenti,
			Tehnicari:      tehnicari,
			SviStatusi:     model.SviStatusi,
			Greska:         "Došlo je do greške pri čuvanju. Pokušajte ponovo.",
			Izmena:         false,
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

	tehnicari, err := h.KorisniciRepo.Lista(r.Context())
	if err != nil {
		http.Error(w, "Greška pri učitavanju tehničara", http.StatusInternalServerError)
		return
	}

	if nalog.GarancijaDo == nil {
		nalog.GarancijaDo = defaultGarancija(nalog.DatumPrijema, podesavanja)
	}
	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "servis"
	ps.NaslovStranice = "Izmeni nalog"
	h.renderujFormuNaloga(w, PodaciFormeNaloga{
		PodaciStranice: ps,
		Nalog:          *nalog,
		Klijenti:       klijenti,
		Tehnicari:      tehnicari,
		SviStatusi:     model.SviStatusi,
		Izmena:         true,
	})
}

// SacuvajIzmenaNaloga prima POST formu i ažurira postojeći servisni nalog
func (h *Handler) SacuvajIzmenaNaloga(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "servis.izmeni"); !ok {
		return
	}
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
		tehnicari, _ := h.KorisniciRepo.Lista(r.Context())
		nalog.ID = id
		ps := h.popuniPodaciStranice(r, podesavanja)
		ps.Stranica = "servis"
		ps.NaslovStranice = "Izmeni nalog"
		h.renderujFormuNaloga(w, PodaciFormeNaloga{
			PodaciStranice: ps,
			Nalog:          nalog,
			Klijenti:       klijenti,
			Tehnicari:      tehnicari,
			SviStatusi:     model.SviStatusi,
			Greska:         greska,
			Izmena:         true,
		})
		return
	}

	nalog.ID = id

	// ako nije izabran postojeći klijent, eventualno kreiraj novog iz polja forme
	if greska := h.mozdaKreirajKlijenta(r.Context(), r, &nalog); greska != "" {
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		klijenti, _ := h.KlijentiRepo.Lista(r.Context(), "")
		tehnicari, _ := h.KorisniciRepo.Lista(r.Context())
		ps := h.popuniPodaciStranice(r, podesavanja)
		ps.Stranica = "servis"
		ps.NaslovStranice = "Izmeni nalog"
		h.renderujFormuNaloga(w, PodaciFormeNaloga{
			PodaciStranice: ps,
			Nalog:          nalog,
			Klijenti:       klijenti,
			Tehnicari:      tehnicari,
			SviStatusi:     model.SviStatusi,
			Greska:         greska,
			Izmena:         true,
		})
		return
	}

	if err := h.ServisRepo.Izmeni(r.Context(), &nalog); err != nil {
		slog.Error("greška pri čuvanju izmene naloga", "error", err)
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		klijenti, _ := h.KlijentiRepo.Lista(r.Context(), "")
		tehnicari, _ := h.KorisniciRepo.Lista(r.Context())
		ps := h.popuniPodaciStranice(r, podesavanja)
		ps.Stranica = "servis"
		ps.NaslovStranice = "Izmeni nalog"
		h.renderujFormuNaloga(w, PodaciFormeNaloga{
			PodaciStranice: ps,
			Nalog:          nalog,
			Klijenti:       klijenti,
			Tehnicari:      tehnicari,
			SviStatusi:     model.SviStatusi,
			Greska:         "Došlo je do greške pri čuvanju. Pokušajte ponovo.",
			Izmena:         true,
		})
		return
	}

	http.Redirect(w, r, "/servis/"+strconv.FormatInt(id, 10)+"?sacuvano=1", http.StatusSeeOther)
}

// ObrisiNalog prima POST zahtev i briše servisni nalog po ID-u
func (h *Handler) ObrisiNalog(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "servis.obrisi"); !ok {
		return
	}
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

// DetaljiNaloga prikazuje sve podatke jednog servisnog naloga sa ugrađenim delovima
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
			klijentNaziv = klijent.PunoIme()
		}
	}

	tehnicarNaziv := ""
	if nalog.TehnicarID != nil {
		tehnicar, err := h.KorisniciRepo.DohvatiPoID(r.Context(), *nalog.TehnicarID)
		if err == nil {
			tehnicarNaziv = tehnicar.KorisnickoIme
		}
	}

	delovi, err := h.ServisniDeloviRepo.DohvatiZaNalog(r.Context(), id)
	if err != nil {
		slog.Error("greška pri učitavanju delova", "error", err)
	}

	appdb := appdbPkg.ArtikalFilter{}
	artikli, err := h.Artikli.Lista(r.Context(), appdb)
	if err != nil {
		slog.Error("greška pri učitavanju artikala", "error", err)
	}

	var ukupnoDelovi float64
	for _, d := range delovi {
		ukupnoDelovi += d.Ukupno()
	}
	var ukupnoSve, preostaloSve float64
	if nalog.CenaKonacna != nil {
		ukupnoSve = *nalog.CenaKonacna + ukupnoDelovi
		avans := 0.0
		if nalog.Avans != nil {
			avans = *nalog.Avans
		}
		preostaloSve = ukupnoSve - avans
		if preostaloSve < 0 {
			preostaloSve = 0
		}
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "servis"
	ps.NaslovStranice = "Detalji naloga"
	podaci := PodaciDetaljiNaloga{
		PodaciStranice: ps,
		Nalog:          *nalog,
		KlijentNaziv:   klijentNaziv,
		TehnicarNaziv:  tehnicarNaziv,
		ServisniDelovi: delovi,
		Artikli:        artikli,
		Sacuvano:       r.URL.Query().Get("sacuvano") == "1",
		UkupnoDelovi:   ukupnoDelovi,
		UkupnoSve:      ukupnoSve,
		PreostaloSve:   preostaloSve,
		SviStatusi:     model.SviStatusi,
	}

	h.renderujTemplate(w, "servis_detalji", podaci)
}

// DodajDeloNalogu prima POST formu i dodaje artikal kao deo servisnog naloga
func (h *Handler) DodajDeloNalogu(w http.ResponseWriter, r *http.Request) {
	k, ok := h.zahtevajDozvolu(w, r, "servis.izmeni")
	if !ok {
		return
	}

	nalogID, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID naloga", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	artikalID, err := strconv.ParseInt(r.FormValue("artikal_id"), 10, 64)
	if err != nil || artikalID <= 0 {
		middleware.SetFlash(w, r, h.DB, "greska", "Neispravan artikal.")
		http.Redirect(w, r, "/servis/"+strconv.FormatInt(nalogID, 10), http.StatusSeeOther)
		return
	}

	kolicina, err := strconv.Atoi(r.FormValue("kolicina"))
	if err != nil || kolicina <= 0 {
		middleware.SetFlash(w, r, h.DB, "greska", "Količina mora biti pozitivan broj.")
		http.Redirect(w, r, "/servis/"+strconv.FormatInt(nalogID, 10), http.StatusSeeOther)
		return
	}

	cena, err := strconv.ParseFloat(r.FormValue("cena_komada"), 64)
	if err != nil || cena < 0 {
		middleware.SetFlash(w, r, h.DB, "greska", "Cena mora biti pozitivan broj.")
		http.Redirect(w, r, "/servis/"+strconv.FormatInt(nalogID, 10), http.StatusSeeOther)
		return
	}

	if _, err := h.ServisniDeloviRepo.Dodaj(r.Context(), nalogID, artikalID, kolicina, cena, &k.ID); err != nil {
		slog.Error("greška pri dodavanju dela", "error", err)
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri dodavanju dela. Proverite stanje na magacinu.")
		http.Redirect(w, r, "/servis/"+strconv.FormatInt(nalogID, 10), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/servis/"+strconv.FormatInt(nalogID, 10)+"?sacuvano=1", http.StatusSeeOther)
}

// ObrisiDeloNaloga prima POST zahtev i uklanja deo iz servisnog naloga
func (h *Handler) ObrisiDeloNaloga(w http.ResponseWriter, r *http.Request) {
	k, ok := h.zahtevajDozvolu(w, r, "servis.izmeni")
	if !ok {
		return
	}

	nalogID, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID naloga", http.StatusBadRequest)
		return
	}

	deoID, err := parseID(chi.URLParam(r, "deo_id"))
	if err != nil {
		http.Error(w, "Neispravan ID dela", http.StatusBadRequest)
		return
	}

	if err := h.ServisniDeloviRepo.Obrisi(r.Context(), deoID, &k.ID); err != nil {
		slog.Error("greška pri brisanju dela", "error", err)
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri uklanjanju dela.")
	}
	http.Redirect(w, r, "/servis/"+strconv.FormatInt(nalogID, 10), http.StatusSeeOther)
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
		Ostecenja:    strings.TrimSpace(r.FormValue("ostecenja")),
		PinUredjaja:  strings.TrimSpace(r.FormValue("pin_uredjaja")),
		Pribor:       strings.TrimSpace(r.FormValue("pribor")),
		DatumPrijema: time.Now(),
	}

	// datum prijema — korisnik može da unese drugi datum (npr. retroaktivno)
	if dp := strings.TrimSpace(r.FormValue("datum_prijema")); dp != "" {
		if t, err := time.Parse("2006-01-02", dp); err == nil {
			nalog.DatumPrijema = t
		}
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

	// opcioni tehničar
	if tidStr := r.FormValue("tehnicar_id"); tidStr != "" {
		if tid, err := strconv.ParseInt(tidStr, 10, 64); err == nil {
			nalog.TehnicarID = &tid
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

	// opcioni datum garancije — preskačemo ako je korisnik označio "bez garancije"
	if r.FormValue("bez_garancije") != "1" {
		if gd := strings.TrimSpace(r.FormValue("garancija_do")); gd != "" {
			if t, err := time.Parse("2006-01-02", gd); err == nil {
				nalog.GarancijaDo = &t
			}
		}
	}

	return nalog, ""
}

// mozdaKreirajKlijenta kreira novog klijenta iz polja forme naloga kada nije
// izabran postojeći (klijent_id prazno), a uneto je ime (fizičko) ili naziv firme
// (pravno). Klijent je opcioni — ako nema dovoljno podataka, nalog ostaje bez njega.
// Vraća srpsku poruku o grešci za prikaz, ili prazan string ako je sve u redu.
func (h *Handler) mozdaKreirajKlijenta(ctx context.Context, r *http.Request, nalog *model.ServisniNalog) string {
	if nalog.KlijentID != nil {
		return "" // izabran postojeći klijent — ništa ne kreiramo
	}

	tip := r.FormValue("tip")
	if tip != "fizicko" && tip != "pravno" {
		tip = "fizicko"
	}
	ime := strings.TrimSpace(r.FormValue("ime"))
	nazivFirme := strings.TrimSpace(r.FormValue("naziv_firme"))

	// bez minimalnih podataka nalog ostaje bez klijenta
	if tip == "fizicko" && ime == "" {
		return ""
	}
	if tip == "pravno" && nazivFirme == "" {
		return ""
	}

	email := strings.TrimSpace(r.FormValue("email"))
	if email != "" && !strings.Contains(email, "@") {
		return "Adresa e-pošte nije ispravna."
	}

	// tip identifikacionog broja: 'jmbg' ili 'licna_karta' (podrazumevano jmbg)
	tipIdent := r.FormValue("tip_identifikacije")
	if tipIdent != "jmbg" && tipIdent != "licna_karta" {
		tipIdent = "jmbg"
	}

	jmbg := strings.TrimSpace(r.FormValue("jmbg"))
	if tipIdent == "jmbg" {
		if greska := validirajJMBG(jmbg); greska != "" {
			return greska
		}
	}

	klijent := model.Klijent{
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
	}

	id, err := h.KlijentiRepo.Kreiraj(ctx, &klijent)
	if err != nil {
		slog.Error("greška pri kreiranju klijenta iz naloga", "error", err)
		return "Došlo je do greške pri čuvanju klijenta. Pokušajte ponovo."
	}
	nalog.KlijentID = &id
	return ""
}

// defaultGarancija vraća datum garancije na osnovu datuma prijema i podešavanja;
// vraća nil ako je rok 0 ili podešavanje nedostaje
func defaultGarancija(datumPrijema time.Time, podesavanja map[string]string) *time.Time {
	meseci, err := strconv.Atoi(vrednostIliDefault(podesavanja, "servis_garancija_meseci", "2"))
	if err != nil || meseci <= 0 {
		return nil
	}
	t := datumPrijema.AddDate(0, meseci, 0)
	return &t
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
func (h *Handler) renderujFormuNaloga(w http.ResponseWriter, podaci PodaciFormeNaloga) {
	h.renderujTemplate(w, "servis_forma", podaci)
}

// PodaciStampeServisa su podaci za print-friendly prikaz servisnog naloga
type PodaciStampeServisa struct {
	Nalog          model.ServisniNalog
	ServisniDelovi []model.ServisniDeoSaArtiklom
	UkupnoDelovi   float64
	KlijentNaziv   string
	TehnicarNaziv  string
	NazivFirme     string
	Podnazlov      string
	Adresa         string
	Telefon        string
	PIB            string
	QRKod          string // base64 PNG QR koda sa URL-om naloga
}

// StampaServisa renderuje print-friendly stranicu za servisni nalog
func (h *Handler) StampaServisa(w http.ResponseWriter, r *http.Request) {
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

	delovi, err := h.ServisniDeloviRepo.DohvatiZaNalog(r.Context(), id)
	if err != nil {
		http.Error(w, "Greška pri učitavanju delova", http.StatusInternalServerError)
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

	tehnicarNaziv := ""
	if nalog.TehnicarID != nil {
		tehnicar, err := h.KorisniciRepo.DohvatiPoID(r.Context(), *nalog.TehnicarID)
		if err == nil {
			tehnicarNaziv = tehnicar.KorisnickoIme
		}
	}

	var ukupnoDelovi float64
	for _, d := range delovi {
		ukupnoDelovi += d.Ukupno()
	}

	// QR kod vodi na javnu status stranicu — dostupnu bez prijave
	nalogURL := qrNalogURL(r, nalog.JavniToken)
	var qrKod string
	if png, err := qrcode.Encode(nalogURL, qrcode.Medium, 160); err == nil {
		qrKod = base64.StdEncoding.EncodeToString(png)
	}

	h.renderujStandalone(w, "servis_stampa", PodaciStampeServisa{
		Nalog:          *nalog,
		ServisniDelovi: delovi,
		UkupnoDelovi:   ukupnoDelovi,
		KlijentNaziv:   klijentNaziv,
		TehnicarNaziv:  tehnicarNaziv,
		NazivFirme:     podesavanja["naziv_firme"],
		Podnazlov:      podesavanja["podnazlov"],
		Adresa:         podesavanja["adresa"],
		Telefon:        podesavanja["telefon"],
		PIB:            podesavanja["pib"],
		QRKod:          qrKod,
	})
}

// PodaciOtpremnice su podaci za otpremnicu pri preuzimanju uređaja
type PodaciOtpremnice struct {
	Nalog          model.ServisniNalog
	ServisniDelovi []model.ServisniDeoSaArtiklom
	UkupnoDelovi   float64
	PreostaloSve   float64
	ImaAvans       bool
	QRKod          string
	Klijent        *model.Klijent
	KlijentNaziv   string
	TehnicarNaziv  string
	NazivFirme     string
	Podnazlov      string
	Adresa         string
	Telefon        string
	PIB            string
}

// StampaOtpremnice renderuje otpremnicu pri preuzimanju uređaja od strane klijenta
func (h *Handler) StampaOtpremnice(w http.ResponseWriter, r *http.Request) {
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

	delovi, err := h.ServisniDeloviRepo.DohvatiZaNalog(r.Context(), id)
	if err != nil {
		http.Error(w, "Greška pri učitavanju delova", http.StatusInternalServerError)
		return
	}

	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	var klijent *model.Klijent
	klijentNaziv := ""
	if nalog.KlijentID != nil {
		k, err := h.KlijentiRepo.DohvatiID(r.Context(), *nalog.KlijentID)
		if err == nil {
			klijent = k
			if k.NazivFirme != "" {
				klijentNaziv = k.NazivFirme
			} else {
				klijentNaziv = strings.TrimSpace(k.Ime + " " + k.Prezime)
			}
		}
	}

	tehnicarNaziv := ""
	if nalog.TehnicarID != nil {
		tehnicar, err := h.KorisniciRepo.DohvatiPoID(r.Context(), *nalog.TehnicarID)
		if err == nil {
			tehnicarNaziv = tehnicar.KorisnickoIme
		}
	}

	var ukupnoDelovi float64
	for _, d := range delovi {
		ukupnoDelovi += d.Ukupno()
	}
	var preostaloSve float64
	var imaAvans bool
	if nalog.CenaKonacna != nil {
		ukupnoSve := *nalog.CenaKonacna + ukupnoDelovi
		avans := 0.0
		if nalog.Avans != nil && *nalog.Avans > 0 {
			avans = *nalog.Avans
			imaAvans = true
		}
		preostaloSve = ukupnoSve - avans
		if preostaloSve < 0 {
			preostaloSve = 0
		}
	}

	nalogURL := qrNalogURL(r, nalog.JavniToken)
	var qrKodOtpr string
	if png, err := qrcode.Encode(nalogURL, qrcode.Medium, 160); err == nil {
		qrKodOtpr = base64.StdEncoding.EncodeToString(png)
	}

	h.renderujStandalone(w, "servis_otpremnica", PodaciOtpremnice{
		Nalog:          *nalog,
		ServisniDelovi: delovi,
		UkupnoDelovi:   ukupnoDelovi,
		PreostaloSve:   preostaloSve,
		ImaAvans:       imaAvans,
		QRKod:          qrKodOtpr,
		Klijent:        klijent,
		KlijentNaziv:   klijentNaziv,
		TehnicarNaziv:  tehnicarNaziv,
		NazivFirme:     podesavanja["naziv_firme"],
		Podnazlov:      podesavanja["podnazlov"],
		Adresa:         podesavanja["adresa"],
		Telefon:        podesavanja["telefon"],
		PIB:            podesavanja["pib"],
	})
}

// PodaciPredracuna su podaci za predračun/ponudu koja se šalje klijentu pre rada
type PodaciPredracuna struct {
	Nalog          model.ServisniNalog
	ServisniDelovi []model.ServisniDeoSaArtiklom
	UkupnoDelovi   float64

	ImaCenuRada bool    // ima li nalog uopšte cenu rada (raspon ili fiksnu)
	CenaRaspon  bool    // true → prikaži procenu Od–Do; false → fiksnu cenu
	CenaRadaOd  float64 // donja granica procene
	CenaRadaDo  float64 // gornja granica procene
	CenaRada    float64 // fiksna cena rada

	UkupnoOd float64 // delovi + CenaRadaOd (kad je raspon)
	UkupnoDo float64 // delovi + CenaRadaDo (kad je raspon)
	Ukupno   float64 // delovi + fiksna cena (ili samo delovi ako nema cene rada)

	DatumIzdavanja time.Time
	VaziDo         time.Time

	QRKod         string
	Klijent       *model.Klijent
	KlijentNaziv  string
	TehnicarNaziv string
	NazivFirme    string
	Podnazlov     string
	Adresa        string
	Telefon       string
	PIB           string
}

// StampaPredracuna renderuje predračun/ponudu koja se šalje klijentu kada se utvrdi kvar
func (h *Handler) StampaPredracuna(w http.ResponseWriter, r *http.Request) {
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

	delovi, err := h.ServisniDeloviRepo.DohvatiZaNalog(r.Context(), id)
	if err != nil {
		http.Error(w, "Greška pri učitavanju delova", http.StatusInternalServerError)
		return
	}

	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	var klijent *model.Klijent
	klijentNaziv := ""
	if nalog.KlijentID != nil {
		k, err := h.KlijentiRepo.DohvatiID(r.Context(), *nalog.KlijentID)
		if err == nil {
			klijent = k
			if k.NazivFirme != "" {
				klijentNaziv = k.NazivFirme
			} else {
				klijentNaziv = strings.TrimSpace(k.Ime + " " + k.Prezime)
			}
		}
	}

	tehnicarNaziv := ""
	if nalog.TehnicarID != nil {
		tehnicar, err := h.KorisniciRepo.DohvatiPoID(r.Context(), *nalog.TehnicarID)
		if err == nil {
			tehnicarNaziv = tehnicar.KorisnickoIme
		}
	}

	var ukupnoDelovi float64
	for _, d := range delovi {
		ukupnoDelovi += d.Ukupno()
	}

	// cena rada: ako postoji procena raspona (Od i Do) prikaži je, inače padni na fiksnu cenu
	var imaCenuRada, cenaRaspon bool
	var cenaRadaOd, cenaRadaDo, cenaRada float64
	var ukupnoOd, ukupnoDo, ukupno float64
	switch {
	case nalog.CenaOd != nil && nalog.CenaDo != nil:
		imaCenuRada = true
		cenaRaspon = true
		cenaRadaOd = *nalog.CenaOd
		cenaRadaDo = *nalog.CenaDo
		ukupnoOd = ukupnoDelovi + cenaRadaOd
		ukupnoDo = ukupnoDelovi + cenaRadaDo
	case nalog.CenaKonacna != nil:
		imaCenuRada = true
		cenaRada = *nalog.CenaKonacna
		ukupno = ukupnoDelovi + cenaRada
	default:
		ukupno = ukupnoDelovi
	}

	// rok važenja iz podešavanja (default 7 dana)
	rok := 7
	if v, err := strconv.Atoi(podesavanja["predracun_rok_dana"]); err == nil && v > 0 {
		rok = v
	}
	datumIzdavanja := time.Now()
	vaziDo := datumIzdavanja.AddDate(0, 0, rok)

	// QR kod vodi na javnu status stranicu — dostupnu bez prijave
	nalogURL := qrNalogURL(r, nalog.JavniToken)
	var qrKod string
	if png, err := qrcode.Encode(nalogURL, qrcode.Medium, 160); err == nil {
		qrKod = base64.StdEncoding.EncodeToString(png)
	}

	h.renderujStandalone(w, "servis_predracun", PodaciPredracuna{
		Nalog:          *nalog,
		ServisniDelovi: delovi,
		UkupnoDelovi:   ukupnoDelovi,
		ImaCenuRada:    imaCenuRada,
		CenaRaspon:     cenaRaspon,
		CenaRadaOd:     cenaRadaOd,
		CenaRadaDo:     cenaRadaDo,
		CenaRada:       cenaRada,
		UkupnoOd:       ukupnoOd,
		UkupnoDo:       ukupnoDo,
		Ukupno:         ukupno,
		DatumIzdavanja: datumIzdavanja,
		VaziDo:         vaziDo,
		QRKod:          qrKod,
		Klijent:        klijent,
		KlijentNaziv:   klijentNaziv,
		TehnicarNaziv:  tehnicarNaziv,
		NazivFirme:     podesavanja["naziv_firme"],
		Podnazlov:      podesavanja["podnazlov"],
		Adresa:         podesavanja["adresa"],
		Telefon:        podesavanja["telefon"],
		PIB:            podesavanja["pib"],
	})
}

// PromeniStatus obrađuje POST /servis/{id}/status i menja samo status naloga
func (h *Handler) PromeniStatus(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID naloga", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}
	noviStatus := strings.TrimSpace(r.FormValue("status"))
	dozvoljenStatusi := map[string]bool{}
	for _, s := range model.SviStatusi {
		dozvoljenStatusi[s] = true
	}
	if !dozvoljenStatusi[noviStatus] {
		http.Error(w, "Nepoznat status", http.StatusBadRequest)
		return
	}
	if err := h.ServisRepo.AzurirajStatus(r.Context(), id, noviStatus); err != nil {
		slog.Error("greška pri promeni statusa naloga", "id", id, "error", err)
		http.Error(w, "Greška pri promeni statusa", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/servis/"+strconv.FormatInt(id, 10)+"?sacuvano=1", http.StatusSeeOther)
}

// PodaciJavnogStatusa su podaci za javnu status stranicu servisnog naloga
type PodaciJavnogStatusa struct {
	Nalog      model.ServisniNalog
	NazivFirme string
	Telefon    string
	Adresa     string
	SviStatusi []string
}

// ServisJavniStatus prikazuje javnu status stranicu — dostupna bez prijave putem QR koda
func (h *Handler) ServisJavniStatus(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		http.NotFound(w, r)
		return
	}

	nalog, err := h.ServisRepo.DohvatiJavniToken(r.Context(), token)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)

	h.renderujStandalone(w, "servis_status_javni", PodaciJavnogStatusa{
		Nalog:      *nalog,
		NazivFirme: podesavanja["naziv_firme"],
		Telefon:    podesavanja["telefon"],
		Adresa:     podesavanja["adresa"],
		SviStatusi: model.SviStatusi,
	})
}

// qrNalogURL konstruiše URL za QR kod vodeći računa o reverse proxy-ju.
// Ako aplikacija radi iza nginx/Caddy/Traefik koji prekida TLS, r.TLS je nil,
// ali X-Forwarded-Proto header sadrži stvarnu šemu.
func qrNalogURL(r *http.Request, token string) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return scheme + "://" + r.Host + "/status/" + token
}
