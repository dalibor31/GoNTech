package handler

import (
	"net/http"
	"strconv"
	"strings"

	"ntech/internal/db"
	"ntech/internal/db/sqlite"
	"ntech/internal/model"

	"github.com/go-chi/chi/v5"
)

// PodaciUsluga su podaci za listu usluga
type PodaciUsluga struct {
	model.PodaciStranice
	Usluge   []model.Usluga
	Pretraga string
	Sacuvano bool
	Obrisan  bool
}

// PodaciFormeUsluge su podaci za formu nove/izmenjene usluge
type PodaciFormeUsluge struct {
	model.PodaciStranice
	Usluga     model.Usluga
	Kategorije []string // postojeće kategorije za predlog (datalist)
	Greska     string
	Izmena     bool
}

// Usluge renderuje listu usluga (cenovnik usluga)
func (h *Handler) Usluge(w http.ResponseWriter, r *http.Request) {
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	pretraga := r.URL.Query().Get("pretraga")
	usluge, err := h.UslugeRepo.Lista(r.Context(), db.UslugaFilter{Pretraga: pretraga})
	if err != nil {
		http.Error(w, "Greška pri učitavanju usluga", http.StatusInternalServerError)
		return
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "usluge"
	ps.NaslovStranice = "Usluge"
	h.renderujTemplate(w, "usluge", PodaciUsluga{
		PodaciStranice: ps,
		Usluge:         usluge,
		Pretraga:       pretraga,
		Sacuvano:       r.URL.Query().Get("sacuvano") == "1",
		Obrisan:        r.URL.Query().Get("obrisan") == "1",
	})
}

// NovaUsluga prikazuje praznu formu sa predloženom auto-šifrom (USL-NNN)
func (h *Handler) NovaUsluga(w http.ResponseWriter, r *http.Request) {
	sifra, err := h.UslugeRepo.SledecaSifra(r.Context())
	if err != nil {
		sifra = "USL-001"
	}
	h.renderujFormuUsluge(w, r, model.Usluga{Sifra: sifra, PdvStopa: h.podrazumevanaPdvStopa(r.Context()), JedinicaMere: "usluga"}, false, "")
}

// IzmeniUslugu prikazuje formu sa postojećom uslugom
func (h *Handler) IzmeniUslugu(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Neispravan ID usluge", http.StatusBadRequest)
		return
	}
	usluga, err := h.UslugeRepo.DohvatiID(r.Context(), id)
	if err != nil {
		http.Error(w, "Usluga nije pronađena", http.StatusNotFound)
		return
	}
	h.renderujFormuUsluge(w, r, *usluga, true, "")
}

// SacuvajUslugu prima POST i kreira novu uslugu
func (h *Handler) SacuvajUslugu(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "artikal.dodaj"); !ok {
		return
	}
	usluga, greska := parseFormuUsluge(r, h.podrazumevanaPdvStopa(r.Context()))
	if greska != "" {
		h.renderujFormuUsluge(w, r, usluga, false, greska)
		return
	}
	if _, err := h.UslugeRepo.Kreiraj(r.Context(), &usluga); err != nil {
		h.renderujFormuUsluge(w, r, usluga, false, "Greška pri čuvanju usluge. Pokušajte ponovo.")
		return
	}
	http.Redirect(w, r, "/usluge?sacuvano=1", http.StatusSeeOther)
}

// SacuvajIzmenuUsluge prima POST i ažurira postojeću uslugu
func (h *Handler) SacuvajIzmenuUsluge(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "artikal.izmeni"); !ok {
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Neispravan ID usluge", http.StatusBadRequest)
		return
	}
	usluga, greska := parseFormuUsluge(r, h.podrazumevanaPdvStopa(r.Context()))
	usluga.ID = id
	if greska != "" {
		h.renderujFormuUsluge(w, r, usluga, true, greska)
		return
	}
	if err := h.UslugeRepo.Izmeni(r.Context(), &usluga); err != nil {
		h.renderujFormuUsluge(w, r, usluga, true, "Greška pri čuvanju usluge. Pokušajte ponovo.")
		return
	}
	http.Redirect(w, r, "/usluge?sacuvano=1", http.StatusSeeOther)
}

// ObrisiUslugu briše uslugu po ID-u
func (h *Handler) ObrisiUslugu(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "artikal.obrisi"); !ok {
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Neispravan ID usluge", http.StatusBadRequest)
		return
	}
	if err := h.UslugeRepo.Obrisi(r.Context(), id); err != nil {
		http.Error(w, "Greška pri brisanju usluge", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/usluge?obrisan=1", http.StatusSeeOther)
}

// parseFormuUsluge čita i validira polja forme usluge. Vraća model i poruku o grešci.
// podrazumevanaStopa je opšta PDV stopa iz šifarnika (v. Handler.podrazumevanaPdvStopa) —
// koristi se samo kad korisnik nije uneo stopu, nikad hardkodovana vrednost.
func parseFormuUsluge(r *http.Request, podrazumevanaStopa float64) (model.Usluga, string) {
	if err := r.ParseForm(); err != nil {
		return model.Usluga{}, "Greška pri čitanju forme."
	}
	naziv := strings.TrimSpace(r.FormValue("naziv"))
	if naziv == "" {
		return model.Usluga{Kategorija: strings.TrimSpace(r.FormValue("kategorija"))}, "Naziv usluge je obavezan."
	}

	cena, err := strconv.ParseFloat(strings.TrimSpace(r.FormValue("cena")), 64)
	if err != nil || cena < 0 {
		cena = 0
	}
	pdv, err := strconv.ParseFloat(strings.TrimSpace(r.FormValue("pdv_stopa")), 64)
	if err != nil || pdv < 0 {
		pdv = podrazumevanaStopa
	}

	jm := strings.TrimSpace(r.FormValue("jedinica_mere"))
	if jm == "" {
		jm = "usluga"
	}

	return model.Usluga{
		Sifra:        strings.TrimSpace(r.FormValue("sifra")),
		Naziv:        naziv,
		Kategorija:   strings.TrimSpace(r.FormValue("kategorija")),
		JedinicaMere: jm,
		Cena:         cena,
		PdvStopa:     pdv,
		Opis:         strings.TrimSpace(r.FormValue("opis")),
	}, ""
}

// renderujFormuUsluge prikazuje formu usluge (zajedničko za novu i izmenu)
func (h *Handler) renderujFormuUsluge(w http.ResponseWriter, r *http.Request, usluga model.Usluga, izmena bool, greska string) {
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}
	kategorije, _ := h.UslugeRepo.Kategorije(r.Context())

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "usluge"
	if izmena {
		ps.NaslovStranice = "Izmena usluge"
	} else {
		ps.NaslovStranice = "Nova usluga"
	}
	h.renderujTemplate(w, "usluge_forma", PodaciFormeUsluge{
		PodaciStranice: ps,
		Usluga:         usluga,
		Kategorije:     kategorije,
		Greska:         greska,
		Izmena:         izmena,
	})
}
