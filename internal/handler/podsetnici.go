package handler

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ntech/internal/db"
	"ntech/internal/db/sqlite"
	"ntech/internal/middleware"
	"ntech/internal/model"

	"github.com/go-chi/chi/v5"
)

// podaciPodsetnici su podaci za stranicu sa listom podsetnika
type podaciPodsetnici struct {
	model.PodaciStranice
	Podsetnici  []model.Podsetnik
	SamoAktivni bool
	Sacuvano    bool
	Obrisan     bool
}

// podaciPodsetnikForma su podaci za formu novog/izmenjenog podsetnika
type podaciPodsetnikForma struct {
	model.PodaciStranice
	Podsetnik model.Podsetnik
	Greska    string
	Izmena    bool
	Korisnici []model.Korisnik
}

// Podsetnici renderuje listu podsetnika
func (h *Handler) Podsetnici(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())

	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	samoAktivni := r.URL.Query().Get("samo_aktivni") == "1"

	// radnik vidi samo podsetnik koji su mu dodeljeni
	filter := db.PodsetnikFilter{SamoAktivni: samoAktivni}
	if k.Uloga == "radnik" {
		filter.KorisnikID = &k.ID
	}

	lista, err := h.PodsetnikRepo.Lista(r.Context(), filter)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podsetnika", http.StatusInternalServerError)
		return
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "podsetnici"
	ps.NaslovStranice = "Podsetnici"

	podaci := podaciPodsetnici{
		PodaciStranice: ps,
		Podsetnici:     lista,
		SamoAktivni:    samoAktivni,
		Sacuvano:       r.URL.Query().Get("sacuvano") == "1",
		Obrisan:        r.URL.Query().Get("obrisan") == "1",
	}

	h.renderujTemplate(w, "podsetnici", podaci)
}

// NoviPodsetnik prikazuje praznu formu za unos novog podsetnika
func (h *Handler) NoviPodsetnik(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())

	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "podsetnici"
	ps.NaslovStranice = "Novi podsetnik"

	var korisnici []model.Korisnik
	if middleware.JeAdmin(k) {
		korisnici, _ = h.KorisniciRepo.Lista(r.Context())
	}

	h.renderujFormuPodsetnika(w, podaciPodsetnikForma{
		PodaciStranice: ps,
		Izmena:         false,
		Korisnici:      korisnici,
	})
}

// SacuvajPodsetnik prima POST formu i upisuje novi podsetnik u bazu
func (h *Handler) SacuvajPodsetnik(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	podsetnik, greska := parseFormuPodsetnika(r, k)

	if greska != "" {
		h.prikaziGreskuPodsetnika(w, r, k, podsetnik, greska, false)
		return
	}

	if _, err := h.PodsetnikRepo.Kreiraj(r.Context(), &podsetnik); err != nil {
		slog.Error("greška pri čuvanju podsetnika", "error", err)
		h.prikaziGreskuPodsetnika(w, r, k, podsetnik, "Došlo je do greške pri čuvanju. Pokušajte ponovo.", false)
		return
	}

	http.Redirect(w, r, "/podsetnici?sacuvano=1", http.StatusSeeOther)
}

// IzmeniPodsetnik učitava podsetnik po ID-u i prikazuje popunjenu formu za izmenu
func (h *Handler) IzmeniPodsetnik(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())

	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID podsetnika", http.StatusBadRequest)
		return
	}

	podsetnik, err := h.PodsetnikRepo.DohvatiID(r.Context(), id)
	if err != nil {
		http.Error(w, "Podsetnik nije pronađen", http.StatusNotFound)
		return
	}

	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "podsetnici"
	ps.NaslovStranice = "Izmeni podsetnik"

	var korisnici []model.Korisnik
	if middleware.JeAdmin(k) {
		korisnici, _ = h.KorisniciRepo.Lista(r.Context())
	}

	h.renderujFormuPodsetnika(w, podaciPodsetnikForma{
		PodaciStranice: ps,
		Podsetnik:      *podsetnik,
		Izmena:         true,
		Korisnici:      korisnici,
	})
}

// SacuvajIzmenePodsetnika prima POST formu i ažurira postojeći podsetnik u bazi
func (h *Handler) SacuvajIzmenePodsetnika(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())

	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID podsetnika", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	podsetnik, greska := parseFormuPodsetnika(r, k)
	podsetnik.ID = id

	if greska != "" {
		h.prikaziGreskuPodsetnika(w, r, k, podsetnik, greska, true)
		return
	}

	if err := h.PodsetnikRepo.Izmeni(r.Context(), &podsetnik); err != nil {
		slog.Error("greška pri čuvanju izmene podsetnika", "error", err)
		h.prikaziGreskuPodsetnika(w, r, k, podsetnik, "Došlo je do greške pri čuvanju. Pokušajte ponovo.", true)
		return
	}

	http.Redirect(w, r, "/podsetnici?sacuvano=1", http.StatusSeeOther)
}

// OznaciPodsetnik prima POST zahtev i menja status završenosti podsetnika
func (h *Handler) OznaciPodsetnik(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID podsetnika", http.StatusBadRequest)
		return
	}

	// učitamo trenutni status pa ga preokrenemo
	podsetnik, err := h.PodsetnikRepo.DohvatiID(r.Context(), id)
	if err != nil {
		http.Error(w, "Podsetnik nije pronađen", http.StatusNotFound)
		return
	}

	if err := h.PodsetnikRepo.OznaciZavrsenim(r.Context(), id, !podsetnik.Zavrseno); err != nil {
		http.Error(w, "Greška pri ažuriranju statusa", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/podsetnici", http.StatusSeeOther)
}

// ObrisiPodsetnik prima POST zahtev i briše podsetnik po ID-u
func (h *Handler) ObrisiPodsetnik(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID podsetnika", http.StatusBadRequest)
		return
	}

	if err := h.PodsetnikRepo.Obrisi(r.Context(), id); err != nil {
		http.Error(w, "Greška pri brisanju podsetnika", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/podsetnici?obrisan=1", http.StatusSeeOther)
}

// parseFormuPodsetnika čita polja iz HTTP forme, validira ih i vraća model i eventualnu grešku
func parseFormuPodsetnika(r *http.Request, k *model.Korisnik) (model.Podsetnik, string) {
	naslov := strings.TrimSpace(r.FormValue("naslov"))
	if naslov == "" {
		return model.Podsetnik{}, "Naslov je obavezan."
	}

	datumStr := strings.TrimSpace(r.FormValue("datum_podsecanja"))
	if datumStr == "" {
		return model.Podsetnik{Naslov: naslov}, "Datum podsećanja je obavezan."
	}

	datum, err := time.Parse("2006-01-02", datumStr)
	if err != nil {
		return model.Podsetnik{Naslov: naslov}, "Datum podsećanja nije u ispravnom formatu."
	}

	p := model.Podsetnik{
		Naslov:          naslov,
		Napomena:        strings.TrimSpace(r.FormValue("napomena")),
		DatumPodsecanja: datum,
		Tip:             model.TipOpsti,
	}

	// admin/superadmin mogu dodeliti podsetnik drugom korisniku
	if middleware.JeAdmin(k) {
		if kidStr := strings.TrimSpace(r.FormValue("korisnik_id")); kidStr != "" {
			if kid, err := strconv.ParseInt(kidStr, 10, 64); err == nil && kid > 0 {
				p.KorisnikID = &kid
			}
		}
	} else {
		// radnik dobija podsetnik dodeljen sebi
		p.KorisnikID = &k.ID
	}

	return p, ""
}

// prikaziGreskuPodsetnika prikazuje formu podsetnika sa porukom o grešci
func (h *Handler) prikaziGreskuPodsetnika(w http.ResponseWriter, r *http.Request, k *model.Korisnik, podsetnik model.Podsetnik, poruka string, izmena bool) {
	podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "podsetnici"
	if izmena {
		ps.NaslovStranice = "Izmeni podsetnik"
	} else {
		ps.NaslovStranice = "Novi podsetnik"
	}
	var korisnici []model.Korisnik
	if middleware.JeAdmin(k) {
		korisnici, _ = h.KorisniciRepo.Lista(r.Context())
	}
	h.renderujFormuPodsetnika(w, podaciPodsetnikForma{
		PodaciStranice: ps,
		Podsetnik:      podsetnik,
		Greska:         poruka,
		Izmena:         izmena,
		Korisnici:      korisnici,
	})
}

// renderujFormuPodsetnika renderuje HTML šablon forme za unos ili izmenu podsetnika
func (h *Handler) renderujFormuPodsetnika(w http.ResponseWriter, podaci podaciPodsetnikForma) {
	h.renderujTemplate(w, "podsetnik_forma", podaci)
}
