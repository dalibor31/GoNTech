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

// PodaciTroskovi su podaci za listu troškova
type PodaciTroskovi struct {
	model.PodaciStranice
	Troskovi []model.Trosak
	Pretraga string
	Sacuvano bool
	Obrisan  bool
}

// PodaciFormeTroska su podaci za formu novog/izmenjenog troška
type PodaciFormeTroska struct {
	model.PodaciStranice
	Trosak model.Trosak
	Greska string
	Izmena bool
}

// Troskovi renderuje listu šifrarnika vrsta troškova
func (h *Handler) Troskovi(w http.ResponseWriter, r *http.Request) {
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	pretraga := r.URL.Query().Get("pretraga")
	troskovi, err := h.TroskoviRepo.Lista(r.Context(), db.TrosakFilter{Pretraga: pretraga})
	if err != nil {
		http.Error(w, "Greška pri učitavanju troškova", http.StatusInternalServerError)
		return
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "troskovi"
	ps.NaslovStranice = "Troškovi"
	h.renderujTemplate(w, "troskovi", PodaciTroskovi{
		PodaciStranice: ps,
		Troskovi:       troskovi,
		Pretraga:       pretraga,
		Sacuvano:       r.URL.Query().Get("sacuvano") == "1",
		Obrisan:        r.URL.Query().Get("obrisan") == "1",
	})
}

// NoviTrosak prikazuje praznu formu sa predloženom auto-šifrom (TRO-NNN)
func (h *Handler) NoviTrosak(w http.ResponseWriter, r *http.Request) {
	sifra, err := h.TroskoviRepo.SledecaSifra(r.Context())
	if err != nil {
		sifra = "TRO-001"
	}
	h.renderujFormuTroska(w, r, model.Trosak{Sifra: sifra}, false, "")
}

// IzmeniTrosak prikazuje formu sa postojećim troškom
func (h *Handler) IzmeniTrosak(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Neispravan ID troška", http.StatusBadRequest)
		return
	}
	trosak, err := h.TroskoviRepo.DohvatiID(r.Context(), id)
	if err != nil {
		http.Error(w, "Trošak nije pronađen", http.StatusNotFound)
		return
	}
	h.renderujFormuTroska(w, r, *trosak, true, "")
}

// SacuvajTrosak prima POST i kreira nov trošak
func (h *Handler) SacuvajTrosak(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "artikal.dodaj"); !ok {
		return
	}
	trosak, greska := parseFormuTroska(r)
	if greska != "" {
		h.renderujFormuTroska(w, r, trosak, false, greska)
		return
	}
	if _, err := h.TroskoviRepo.Kreiraj(r.Context(), &trosak); err != nil {
		h.renderujFormuTroska(w, r, trosak, false, "Greška pri čuvanju troška. Pokušajte ponovo.")
		return
	}
	http.Redirect(w, r, "/troskovi?sacuvano=1", http.StatusSeeOther)
}

// SacuvajIzmenuTroska prima POST i ažurira postojeći trošak
func (h *Handler) SacuvajIzmenuTroska(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "artikal.izmeni"); !ok {
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Neispravan ID troška", http.StatusBadRequest)
		return
	}
	trosak, greska := parseFormuTroska(r)
	trosak.ID = id
	if greska != "" {
		h.renderujFormuTroska(w, r, trosak, true, greska)
		return
	}
	if err := h.TroskoviRepo.Izmeni(r.Context(), &trosak); err != nil {
		h.renderujFormuTroska(w, r, trosak, true, "Greška pri čuvanju troška. Pokušajte ponovo.")
		return
	}
	http.Redirect(w, r, "/troskovi?sacuvano=1", http.StatusSeeOther)
}

// ObrisiTrosak briše trošak po ID-u
func (h *Handler) ObrisiTrosak(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "artikal.obrisi"); !ok {
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Neispravan ID troška", http.StatusBadRequest)
		return
	}
	if err := h.TroskoviRepo.Obrisi(r.Context(), id); err != nil {
		http.Error(w, "Greška pri brisanju troška", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/troskovi?obrisan=1", http.StatusSeeOther)
}

// parseFormuTroska čita i validira polja forme troška
func parseFormuTroska(r *http.Request) (model.Trosak, string) {
	if err := r.ParseForm(); err != nil {
		return model.Trosak{}, "Greška pri čitanju forme."
	}
	naziv := strings.TrimSpace(r.FormValue("naziv"))
	if naziv == "" {
		return model.Trosak{}, "Naziv troška je obavezan."
	}
	cena, err := strconv.ParseFloat(strings.TrimSpace(r.FormValue("cena")), 64)
	if err != nil || cena < 0 {
		cena = 0
	}
	return model.Trosak{
		Sifra: strings.TrimSpace(r.FormValue("sifra")),
		Naziv: naziv,
		Cena:  cena,
		Opis:  strings.TrimSpace(r.FormValue("opis")),
	}, ""
}

// renderujFormuTroska prikazuje formu troška (zajedničko za nov i izmenu)
func (h *Handler) renderujFormuTroska(w http.ResponseWriter, r *http.Request, trosak model.Trosak, izmena bool, greska string) {
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}
	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "troskovi"
	if izmena {
		ps.NaslovStranice = "Izmena troška"
	} else {
		ps.NaslovStranice = "Nov trošak"
	}
	h.renderujTemplate(w, "troskovi_forma", PodaciFormeTroska{
		PodaciStranice: ps,
		Trosak:         trosak,
		Greska:         greska,
		Izmena:         izmena,
	})
}
