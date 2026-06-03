package handler

import (
	"encoding/json"
	"errors"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	appdb "ntech/internal/db"
	"ntech/internal/db/sqlite"
	"ntech/internal/model"

	"github.com/go-chi/chi/v5"
)

// PodaciProdaje su podaci za stranicu sa listom prodajnih naloga
type PodaciProdaje struct {
	model.PodaciStranice
	Nalozi   []model.ProdajniNalogSaDetaljem
	Sacuvano bool
	Obrisan  bool
	Pretraga string
}

// PodaciFormeProdaje su podaci za formu unosa nove prodaje
type PodaciFormeProdaje struct {
	model.PodaciStranice
	Artikli     []model.ArtikalSaKategorijom
	ArtikliJSON template.JS
	Klijenti    []model.Klijent
	Greska      string
}

// PodaciDetaljiProdaje su podaci za pregled jedne prodaje sa stavkama
type PodaciDetaljiProdaje struct {
	model.PodaciStranice
	Nalog        model.ProdajniNalog
	Stavke       []model.StavkaProdajeSaArtiklom
	KlijentNaziv string
	Sacuvano     bool
}

// PodaciStampeProdaje su podaci za stranicu za štampanje priznanice
type PodaciStampeProdaje struct {
	Nalog        model.ProdajniNalog
	Stavke       []model.StavkaProdajeSaArtiklom
	KlijentNaziv string
	NazivFirme   string
	Podnazlov    string
	Adresa       string
	Telefon      string
	PIB          string
}

// artikalUJSONSaCenom pretvara listu artikala u template.JS vrednost sa prodajnom cenom i stanjem
func artikalUJSONSaCenom(artikli []model.ArtikalSaKategorijom) template.JS {
	type stavka struct {
		ID       int64   `json:"id"`
		Naziv    string  `json:"naziv"`
		Cena     float64 `json:"cena"`
		Kolicina int     `json:"kolicina"`
	}
	lista := make([]stavka, 0, len(artikli))
	for _, a := range artikli {
		lista = append(lista, stavka{ID: a.ID, Naziv: a.Naziv, Cena: a.ProdajnaCena, Kolicina: a.Kolicina})
	}
	b, _ := json.Marshal(lista)
	return template.JS(b)
}

// Prodaja renderuje listu svih prodajnih naloga
func (h *Handler) Prodaja(w http.ResponseWriter, r *http.Request) {
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	pretraga := strings.TrimSpace(r.URL.Query().Get("pretraga"))

	nalozi, err := h.ProdajaRepo.Lista(r.Context(), pretraga)
	if err != nil {
		http.Error(w, "Greška pri učitavanju prodaje", http.StatusInternalServerError)
		return
	}

	podaci := PodaciProdaje{
		PodaciStranice: model.PodaciStranice{
			Stranica:       "prodaja",
			NaslovStranice: "Prodaja",
			Tema:           podesavanja["tema"],
			NazivFirme:     podesavanja["naziv_firme"],
			Podnazlov:      podesavanja["podnazlov"],
			LogoTip:        podesavanja["logo_tip"],
			LogoPutanja:    podesavanja["logo_putanja"],
			Korisnik:       "Admin",
		},
		Nalozi:   nalozi,
		Sacuvano: r.URL.Query().Get("sacuvano") == "1",
		Obrisan:  r.URL.Query().Get("obrisan") == "1",
		Pretraga: pretraga,
	}

	h.renderujTemplate(w, "prodaja", podaci)
}

// NovaProdaja prikazuje formu za unos novog prodajnog naloga
func (h *Handler) NovaProdaja(w http.ResponseWriter, r *http.Request) {
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	artikli, err := h.Artikli.Lista(r.Context(), appdb.ArtikalFilter{})
	if err != nil {
		http.Error(w, "Greška pri učitavanju artikala", http.StatusInternalServerError)
		return
	}

	klijenti, err := h.KlijentiRepo.Lista(r.Context(), "")
	if err != nil {
		http.Error(w, "Greška pri učitavanju klijenata", http.StatusInternalServerError)
		return
	}

	h.renderujFormuProdaje(w, PodaciFormeProdaje{
		PodaciStranice: model.PodaciStranice{
			Stranica:       "prodaja",
			NaslovStranice: "Nova prodaja",
			Tema:           podesavanja["tema"],
			NazivFirme:     podesavanja["naziv_firme"],
			Podnazlov:      podesavanja["podnazlov"],
			LogoTip:        podesavanja["logo_tip"],
			LogoPutanja:    podesavanja["logo_putanja"],
			Korisnik:       "Admin",
		},
		Artikli:     artikli,
		ArtikliJSON: artikalUJSONSaCenom(artikli),
		Klijenti:    klijenti,
	})
}

// SacuvajProdaju prima POST formu, parsira stavke i upisuje prodajni nalog u bazu
func (h *Handler) SacuvajProdaju(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	nalog, stavke, greska := parseFormuProdaje(r)

	renderujGresku := func(poruka string) {
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		artikli, _ := h.Artikli.Lista(r.Context(), appdb.ArtikalFilter{})
		klijenti, _ := h.KlijentiRepo.Lista(r.Context(), "")
		h.renderujFormuProdaje(w, PodaciFormeProdaje{
			PodaciStranice: model.PodaciStranice{
				Stranica:       "prodaja",
				NaslovStranice: "Nova prodaja",
				Tema:           podesavanja["tema"],
				NazivFirme:     podesavanja["naziv_firme"],
				Podnazlov:      podesavanja["podnazlov"],
				LogoTip:        podesavanja["logo_tip"],
				LogoPutanja:    podesavanja["logo_putanja"],
				Korisnik:       "Admin",
			},
			Artikli:     artikli,
			ArtikliJSON: artikalUJSONSaCenom(artikli),
			Klijenti:    klijenti,
			Greska:      poruka,
		})
	}

	if greska != "" {
		renderujGresku(greska)
		return
	}

	brojNaloga, err := h.ProdajaRepo.SledeciBroj(r.Context())
	if err != nil {
		log.Printf("greška pri generisanju broja naloga: %v", err)
		renderujGresku("Greška pri generisanju broja naloga.")
		return
	}

	nalog.BrojNaloga = brojNaloga
	nalog.Datum = time.Now()

	var ukupno float64
	for _, s := range stavke {
		ukupno += float64(s.Kolicina) * s.CenaPoKomadu
	}
	nalog.Ukupno = ukupno

	id, err := h.ProdajaRepo.Kreiraj(r.Context(), &nalog, stavke)
	if err != nil {
		var errStanje *appdb.ErrNedovoljnoKolicine
		if errors.As(err, &errStanje) {
			renderujGresku(errStanje.Error())
		} else {
			log.Printf("greška pri čuvanju prodaje: %v", err)
			renderujGresku("Greška pri čuvanju prodajnog naloga.")
		}
		return
	}

	http.Redirect(w, r, "/prodaja/"+strconv.FormatInt(id, 10)+"?sacuvano=1", http.StatusSeeOther)
}

// DetaljiProdaje prikazuje pregled jednog prodajnog naloga sa svim stavkama
func (h *Handler) DetaljiProdaje(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID naloga", http.StatusBadRequest)
		return
	}

	nalog, err := h.ProdajaRepo.DohvatiID(r.Context(), id)
	if err != nil {
		http.Error(w, "Nalog nije pronađen", http.StatusNotFound)
		return
	}

	stavke, err := h.ProdajaRepo.DohvatiStavke(r.Context(), id)
	if err != nil {
		http.Error(w, "Greška pri učitavanju stavki", http.StatusInternalServerError)
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

	podaci := PodaciDetaljiProdaje{
		PodaciStranice: model.PodaciStranice{
			Stranica:       "prodaja",
			NaslovStranice: "Detalji prodaje",
			Tema:           podesavanja["tema"],
			NazivFirme:     podesavanja["naziv_firme"],
			Podnazlov:      podesavanja["podnazlov"],
			LogoTip:        podesavanja["logo_tip"],
			LogoPutanja:    podesavanja["logo_putanja"],
			Korisnik:       "Admin",
		},
		Nalog:        *nalog,
		Stavke:       stavke,
		KlijentNaziv: klijentNaziv,
		Sacuvano:     r.URL.Query().Get("sacuvano") == "1",
	}

	h.renderujTemplate(w, "prodaja_detalji", podaci)
}

// StampaProdaje renderuje print-friendly stranicu za dati prodajni nalog
func (h *Handler) StampaProdaje(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID naloga", http.StatusBadRequest)
		return
	}

	nalog, err := h.ProdajaRepo.DohvatiID(r.Context(), id)
	if err != nil {
		http.Error(w, "Nalog nije pronađen", http.StatusNotFound)
		return
	}

	stavke, err := h.ProdajaRepo.DohvatiStavke(r.Context(), id)
	if err != nil {
		http.Error(w, "Greška pri učitavanju stavki", http.StatusInternalServerError)
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

	podaci := PodaciStampeProdaje{
		Nalog:        *nalog,
		Stavke:       stavke,
		KlijentNaziv: klijentNaziv,
		NazivFirme:   podesavanja["naziv_firme"],
		Podnazlov:    podesavanja["podnazlov"],
		Adresa:       podesavanja["adresa"],
		Telefon:      podesavanja["telefon"],
		PIB:          podesavanja["pib"],
	}

	h.renderujStandalone(w, "prodaja_stampa", podaci)
}

// ObrisiProdaju prima POST zahtev, vraća stanje na magacin i briše nalog
func (h *Handler) ObrisiProdaju(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID naloga", http.StatusBadRequest)
		return
	}

	if err := h.ProdajaRepo.Obrisi(r.Context(), id); err != nil {
		http.Error(w, "Greška pri brisanju naloga", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/prodaja?obrisan=1", http.StatusSeeOther)
}

// parseFormuProdaje čita zaglavlje i stavke iz HTTP forme i vraća model i eventualnu grešku
func parseFormuProdaje(r *http.Request) (model.ProdajniNalog, []model.StavkaProdaje, string) {
	var nalog model.ProdajniNalog

	if klijentIDStr := r.FormValue("klijent_id"); klijentIDStr != "" {
		id, err := strconv.ParseInt(klijentIDStr, 10, 64)
		if err == nil {
			nalog.KlijentID = &id
		}
	}
	nalog.Napomena = strings.TrimSpace(r.FormValue("napomena"))

	artikalIDovi := r.Form["artikal_id[]"]
	kolicine := r.Form["kolicina[]"]
	cene := r.Form["cena_po_komadu[]"]

	if len(artikalIDovi) == 0 {
		return nalog, nil, "Prodaja mora imati najmanje jednu stavku."
	}

	if len(artikalIDovi) != len(kolicine) || len(artikalIDovi) != len(cene) {
		return nalog, nil, "Greška u podacima forme — broj stavki nije ispravan."
	}

	var stavke []model.StavkaProdaje
	for i := range artikalIDovi {
		artikalID, err := strconv.ParseInt(strings.TrimSpace(artikalIDovi[i]), 10, 64)
		if err != nil || artikalID <= 0 {
			return nalog, nil, "Neispravan artikal u stavci."
		}

		kolicina, err := strconv.Atoi(strings.TrimSpace(kolicine[i]))
		if err != nil || kolicina <= 0 {
			return nalog, nil, "Količina mora biti pozitivan broj."
		}

		cena, err := strconv.ParseFloat(strings.TrimSpace(cene[i]), 64)
		if err != nil || cena < 0 {
			return nalog, nil, "Cena mora biti pozitivan broj."
		}

		stavke = append(stavke, model.StavkaProdaje{
			ArtikalID:    artikalID,
			Kolicina:     kolicina,
			CenaPoKomadu: cena,
		})
	}

	return nalog, stavke, ""
}

// renderujFormuProdaje renderuje HTML šablon forme za unos nove prodaje
func (h *Handler) renderujFormuProdaje(w http.ResponseWriter, podaci PodaciFormeProdaje) {
	h.renderujTemplate(w, "prodaja_forma", podaci)
}
