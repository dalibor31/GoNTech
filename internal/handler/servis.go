package handler

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	appdbPkg "ntech/internal/db"
	"ntech/internal/db/sqlite"
	"ntech/internal/middleware"
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
	h.renderujFormuNaloga(w, PodaciFormeNaloga{
		PodaciStranice: ps,
		Nalog:          model.ServisniNalog{BrojNaloga: brojNaloga, Status: model.StatusPrimljeno},
		Klijenti:       klijenti,
		Tehnicari:      tehnicari,
		SviStatusi:     model.SviStatusi,
		Izmena:         false,
	})
}

// SacuvajNalog prima POST formu i upisuje novi servisni nalog u bazu
func (h *Handler) SacuvajNalog(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if !h.DozvoleRepo.ImaDozvolu(r.Context(), k.Uloga, "servis.dodaj") {
		http.Error(w, "Nemate dozvolu za ovu akciju.", http.StatusForbidden)
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

	id, err := h.ServisRepo.Kreiraj(r.Context(), &nalog)
	if err != nil {
		log.Printf("greška pri čuvanju naloga: %v", err)
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
	k := middleware.KorisnikIzKonteksta(r.Context())
	if !h.DozvoleRepo.ImaDozvolu(r.Context(), k.Uloga, "servis.izmeni") {
		http.Error(w, "Nemate dozvolu za ovu akciju.", http.StatusForbidden)
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
	if err := h.ServisRepo.Izmeni(r.Context(), &nalog); err != nil {
		log.Printf("greška pri čuvanju izmene naloga: %v", err)
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
	k := middleware.KorisnikIzKonteksta(r.Context())
	if !h.DozvoleRepo.ImaDozvolu(r.Context(), k.Uloga, "servis.obrisi") {
		http.Error(w, "Nemate dozvolu za ovu akciju.", http.StatusForbidden)
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
		log.Printf("greška pri učitavanju delova: %v", err)
	}

	appdb := appdbPkg.ArtikalFilter{}
	artikli, err := h.Artikli.Lista(r.Context(), appdb)
	if err != nil {
		log.Printf("greška pri učitavanju artikala: %v", err)
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
	}

	h.renderujTemplate(w, "servis_detalji", podaci)
}

// DodajDeloNalogu prima POST formu i dodaje artikal kao deo servisnog naloga
func (h *Handler) DodajDeloNalogu(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if !h.DozvoleRepo.ImaDozvolu(r.Context(), k.Uloga, "servis.izmeni") {
		http.Error(w, "Nemate dozvolu za ovu akciju.", http.StatusForbidden)
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
		log.Printf("greška pri dodavanju dela: %v", err)
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri dodavanju dela. Proverite stanje na magacinu.")
		http.Redirect(w, r, "/servis/"+strconv.FormatInt(nalogID, 10), http.StatusSeeOther)
		return
	}

	middleware.SetFlash(w, r, h.DB, "uspeh", "Deo je dodat.")
	http.Redirect(w, r, "/servis/"+strconv.FormatInt(nalogID, 10), http.StatusSeeOther)
}

// ObrisiDeloNaloga prima POST zahtev i uklanja deo iz servisnog naloga
func (h *Handler) ObrisiDeloNaloga(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if !h.DozvoleRepo.ImaDozvolu(r.Context(), k.Uloga, "servis.izmeni") {
		http.Error(w, "Nemate dozvolu za ovu akciju.", http.StatusForbidden)
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
		log.Printf("greška pri brisanju dela: %v", err)
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri uklanjanju dela.")
	} else {
		middleware.SetFlash(w, r, h.DB, "uspeh", "Deo je uklonjen.")
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

	// opcioni datum garancije
	if gd := strings.TrimSpace(r.FormValue("garancija_do")); gd != "" {
		if t, err := time.Parse("2006-01-02", gd); err == nil {
			nalog.GarancijaDo = &t
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
func (h *Handler) renderujFormuNaloga(w http.ResponseWriter, podaci PodaciFormeNaloga) {
	h.renderujTemplate(w, "servis_forma", podaci)
}
