package handler

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"ntech/internal/db"
	"ntech/internal/db/sqlite"
	"ntech/internal/middleware"
	"ntech/internal/model"

	"github.com/go-chi/chi/v5"
)

// PodaciNabavki su podaci za stranicu sa listom nabavki
type PodaciNabavki struct {
	model.PodaciStranice
	Nabavke  []model.NabavkaSaDetaljem
	Sacuvano bool
	Obrisan  bool
}

// PodaciFormeNabavke su podaci za formu unosa nove nabavke
type PodaciFormeNabavke struct {
	model.PodaciStranice
	Artikli     []model.ArtikalSaKategorijom
	ArtikliJSON template.JS        // JSON niz artikala za Alpine.js — bezbedan za umetanje u <script>
	Dobavljaci  []model.Dobavljac
	Kategorije  []model.Kategorija // za dropdown u modalu novog artikla
	Greska      string
}

// PodaciDetaljiNabavke su podaci za pregled jedne nabavke sa stavkama
type PodaciDetaljiNabavke struct {
	model.PodaciStranice
	Nabavka        model.Nabavka
	Stavke         []model.StavkaSaArtiklom
	DobavljacNaziv string
}

// artikalUJSON pretvara listu artikala u template.JS vrednost bezbednu za umetanje u <script> tag
func artikalUJSON(artikli []model.ArtikalSaKategorijom) template.JS {
	type stavka struct {
		ID    int64  `json:"id"`
		Naziv string `json:"naziv"`
	}
	lista := make([]stavka, 0, len(artikli))
	for _, a := range artikli {
		lista = append(lista, stavka{ID: a.ID, Naziv: a.Naziv})
	}
	b, _ := json.Marshal(lista)
	return template.JS(b)
}

// Nabavke renderuje listu svih nabavki
func (h *Handler) Nabavke(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if !h.DozvoleRepo.ImaDozvolu(r.Context(), k.Uloga, "nabavka.pregled") {
		http.Error(w, "Nemate dozvolu za pregled nabavki.", http.StatusForbidden)
		return
	}

	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	nabavke, err := h.NabavkeRepo.Lista(r.Context())
	if err != nil {
		http.Error(w, "Greška pri učitavanju nabavki", http.StatusInternalServerError)
		return
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "nabavke"
	ps.NaslovStranice = "Nabavke"
	podaci := PodaciNabavki{
		PodaciStranice: ps,
		Nabavke:        nabavke,
		Sacuvano:       r.URL.Query().Get("sacuvano") == "1",
		Obrisan:        r.URL.Query().Get("obrisan") == "1",
	}

	h.renderujTemplate(w, "nabavke", podaci)
}

// NovaNabavka prikazuje formu za unos nove nabavke
func (h *Handler) NovaNabavka(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if !h.DozvoleRepo.ImaDozvolu(r.Context(), k.Uloga, "nabavka.pregled") {
		http.Error(w, "Nemate dozvolu za ovu akciju.", http.StatusForbidden)
		return
	}

	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	artikli, err := h.Artikli.Lista(r.Context(), db.ArtikalFilter{})
	if err != nil {
		http.Error(w, "Greška pri učitavanju artikala", http.StatusInternalServerError)
		return
	}

	dobavljaci, err := h.DobavljaciRepo.Lista(r.Context(), "")
	if err != nil {
		http.Error(w, "Greška pri učitavanju dobavljača", http.StatusInternalServerError)
		return
	}

	kategorije, err := h.KategorijeRepo.Lista(r.Context())
	if err != nil {
		http.Error(w, "Greška pri učitavanju kategorija", http.StatusInternalServerError)
		return
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "nabavke"
	ps.NaslovStranice = "Nova nabavka"
	h.renderujFormuNabavke(w, PodaciFormeNabavke{
		PodaciStranice: ps,
		Artikli:        artikli,
		ArtikliJSON:    artikalUJSON(artikli),
		Dobavljaci:     dobavljaci,
		Kategorije:     kategorije,
	})
}

// SacuvajNabavku prima POST formu, parsira stavke i upisuje nabavku u bazu
func (h *Handler) SacuvajNabavku(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if !h.DozvoleRepo.ImaDozvolu(r.Context(), k.Uloga, "nabavka.dodaj") {
		http.Error(w, "Nemate dozvolu za ovu akciju.", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	nabavka, stavke, greska := parseFormuNabavke(r)
	if greska != "" {
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		artikli, _ := h.Artikli.Lista(r.Context(), db.ArtikalFilter{})
		dobavljaci, _ := h.DobavljaciRepo.Lista(r.Context(), "")
		kategorije, _ := h.KategorijeRepo.Lista(r.Context())
		ps := h.popuniPodaciStranice(r, podesavanja)
		ps.Stranica = "nabavke"
		ps.NaslovStranice = "Nova nabavka"
		h.renderujFormuNabavke(w, PodaciFormeNabavke{
			PodaciStranice: ps,
			Artikli:        artikli,
			ArtikliJSON:    artikalUJSON(artikli),
			Dobavljaci:     dobavljaci,
			Kategorije:     kategorije,
			Greska:         greska,
		})
		return
	}

	id, err := h.NabavkeRepo.Kreiraj(r.Context(), &nabavka, stavke)
	if err != nil {
		http.Error(w, "Greška pri čuvanju nabavke", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/nabavke/"+strconv.FormatInt(id, 10)+"?sacuvano=1", http.StatusSeeOther)
}

// DetaljiNabavke prikazuje pregled jedne nabavke sa svim stavkama
func (h *Handler) DetaljiNabavke(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if !h.DozvoleRepo.ImaDozvolu(r.Context(), k.Uloga, "nabavka.pregled") {
		http.Error(w, "Nemate dozvolu za ovu akciju.", http.StatusForbidden)
		return
	}

	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID nabavke", http.StatusBadRequest)
		return
	}

	nabavka, err := h.NabavkeRepo.DohvatiID(r.Context(), id)
	if err != nil {
		http.Error(w, "Nabavka nije pronađena", http.StatusNotFound)
		return
	}

	stavke, err := h.NabavkeRepo.DohvatiStavke(r.Context(), id)
	if err != nil {
		http.Error(w, "Greška pri učitavanju stavki", http.StatusInternalServerError)
		return
	}

	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	// naziv dobavljača dohvatamo samo ako nabavka ima dobavljača
	dobavljacNaziv := ""
	if nabavka.DobavljacID != nil {
		dobavljac, err := h.DobavljaciRepo.DohvatiID(r.Context(), *nabavka.DobavljacID)
		if err == nil {
			dobavljacNaziv = dobavljac.Naziv
		}
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "nabavke"
	ps.NaslovStranice = "Detalji nabavke"
	podaci := PodaciDetaljiNabavke{
		PodaciStranice: ps,
		Nabavka:        *nabavka,
		Stavke:         stavke,
		DobavljacNaziv: dobavljacNaziv,
	}

	h.renderujTemplate(w, "nabavka_detalji", podaci)
}

// ObrisiNabavku prima POST zahtev i briše nabavku po ID-u
func (h *Handler) ObrisiNabavku(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if !h.DozvoleRepo.ImaDozvolu(r.Context(), k.Uloga, "nabavka.obrisi") {
		http.Error(w, "Nemate dozvolu za ovu akciju.", http.StatusForbidden)
		return
	}
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID nabavke", http.StatusBadRequest)
		return
	}

	if err := h.NabavkeRepo.Obrisi(r.Context(), id); err != nil {
		http.Error(w, "Greška pri brisanju nabavke", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/nabavke?obrisan=1", http.StatusSeeOther)
}

// parseFormuNabavke čita zaglavlje i stavke iz HTTP forme i vraća model i eventualnu grešku
func parseFormuNabavke(r *http.Request) (model.Nabavka, []model.StavkaNabavke, string) {
	var nabavka model.Nabavka

	// opcioni dobavljač
	if dobavljacIDStr := r.FormValue("dobavljac_id"); dobavljacIDStr != "" {
		id, err := strconv.ParseInt(dobavljacIDStr, 10, 64)
		if err == nil {
			nabavka.DobavljacID = &id
		}
	}
	nabavka.Napomena = strings.TrimSpace(r.FormValue("napomena"))

	// paralelni nizovi stavki
	artikalIDovi := r.Form["artikal_id[]"]
	kolicine := r.Form["kolicina[]"]
	cene := r.Form["cena_po_komadu[]"]

	if len(artikalIDovi) == 0 {
		return nabavka, nil, "Nabavka mora imati najmanje jednu stavku."
	}

	if len(artikalIDovi) != len(kolicine) || len(artikalIDovi) != len(cene) {
		return nabavka, nil, "Greška u podacima forme — broj stavki nije ispravan."
	}

	var stavke []model.StavkaNabavke
	for i := range artikalIDovi {
		artikalID, err := strconv.ParseInt(strings.TrimSpace(artikalIDovi[i]), 10, 64)
		if err != nil || artikalID <= 0 {
			return nabavka, nil, "Neispravan artikal u stavci."
		}

		kolicina, err := strconv.Atoi(strings.TrimSpace(kolicine[i]))
		if err != nil || kolicina <= 0 {
			return nabavka, nil, "Količina mora biti pozitivan broj."
		}

		cena, err := strconv.ParseFloat(strings.TrimSpace(cene[i]), 64)
		if err != nil || cena < 0 {
			return nabavka, nil, "Cena mora biti pozitivan broj."
		}

		stavke = append(stavke, model.StavkaNabavke{
			ArtikalID:    artikalID,
			Kolicina:     kolicina,
			CenaPoKomadu: cena,
		})
	}

	return nabavka, stavke, ""
}

// renderujFormuNabavke renderuje HTML šablon forme za unos nove nabavke
func (h *Handler) renderujFormuNabavke(w http.ResponseWriter, podaci PodaciFormeNabavke) {
	h.renderujTemplate(w, "nabavka_forma", podaci)
}
