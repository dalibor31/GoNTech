package handler

import (
	"net/http"
	"strconv"

	"ntech/internal/db"
	"ntech/internal/db/sqlite"
	"ntech/internal/model"

	"github.com/go-chi/chi/v5"
)

// PodaciMagacina su podaci za stranicu magacina
type PodaciMagacina struct {
	model.PodaciStranice
	Artikli          []model.ArtikalSaKategorijom
	Kategorije       []model.Kategorija
	Filter           db.ArtikalFilter
	KategorijaIDStr  string
	Sacuvano         bool
	Obrisan          bool
	Premesten        bool
	StranicaBr       int
	UkupnoStranica   int
	UkupnoArtikala   int
	StranicaPrev     int
	StranicaNext     int
	StranicaQueryUrl string // čuva filtere za linkove paginacije
}

// Magacin renderuje listu artikala
func (h *Handler) Magacin(w http.ResponseWriter, r *http.Request) {
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	filter := db.ArtikalFilter{
		Pretraga:     r.URL.Query().Get("pretraga"),
		SamoKriticni: r.URL.Query().Get("kriticni") == "1",
	}

	katIDStr := ""
	if katID := r.URL.Query().Get("kategorija"); katID != "" {
		id, err := strconv.ParseInt(katID, 10, 64)
		if err == nil {
			filter.KategorijaID = &id
			katIDStr = katID
		}
	}

	const pageSize = 100
	stranicaBr := 1
	if p := r.URL.Query().Get("stranica"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			stranicaBr = v
		}
	}
	filter.Limit = pageSize
	filter.Offset = (stranicaBr - 1) * pageSize

	artikli, err := h.Artikli.Lista(r.Context(), filter)
	if err != nil {
		http.Error(w, "Greška pri učitavanju artikala", http.StatusInternalServerError)
		return
	}

	ukupno, err := h.Artikli.PrebrojiPoFilteru(r.Context(), filter)
	if err != nil {
		http.Error(w, "Greška pri učitavanju artikala", http.StatusInternalServerError)
		return
	}

	ukupnoStranica := (ukupno + pageSize - 1) / pageSize

	kategorije, err := h.KategorijeRepo.Lista(r.Context())
	if err != nil {
		http.Error(w, "Greška pri učitavanju kategorija", http.StatusInternalServerError)
		return
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "magacin"
	ps.NaslovStranice = "Magacin"

	// izgradi query string za paginaciju (čuva filtere)
	queryDelići := ""
	if v := filter.Pretraga; v != "" {
		queryDelići += "&pretraga=" + v
	}
	if katIDStr != "" {
		queryDelići += "&kategorija=" + katIDStr
	}
	if filter.SamoKriticni {
		queryDelići += "&kriticni=1"
	}

	stranicaPrev := stranicaBr - 1
	if stranicaPrev < 1 {
		stranicaPrev = 1
	}
	stranicaNext := stranicaBr + 1
	if stranicaNext > ukupnoStranica {
		stranicaNext = ukupnoStranica
	}

	podaci := PodaciMagacina{
		PodaciStranice:   ps,
		Artikli:          artikli,
		Kategorije:       kategorije,
		Filter:           filter,
		KategorijaIDStr:  katIDStr,
		Sacuvano:         r.URL.Query().Get("sacuvano") == "1",
		Obrisan:          r.URL.Query().Get("obrisan") == "1",
		Premesten:        r.URL.Query().Get("premesten") == "1",
		StranicaBr:       stranicaBr,
		UkupnoStranica:   ukupnoStranica,
		UkupnoArtikala:   ukupno,
		StranicaPrev:     stranicaPrev,
		StranicaNext:     stranicaNext,
		StranicaQueryUrl: queryDelići,
	}

	h.renderujTemplate(w, "magacin", podaci)
}

// PremestiArtikal menja kategoriju artikla (premeštanje u drugu kategoriju).
// Prazno polje kategorija_id znači premeštanje u "bez kategorije".
func (h *Handler) PremestiArtikal(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "artikal.premesti"); !ok {
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Neispravan ID artikla", http.StatusBadRequest)
		return
	}

	var kategorijaID *int64
	if v := r.FormValue("kategorija_id"); v != "" {
		kid, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			http.Error(w, "Neispravna kategorija", http.StatusBadRequest)
			return
		}
		kategorijaID = &kid
	}

	if err := h.Artikli.PremestiKategoriju(r.Context(), id, kategorijaID); err != nil {
		http.Error(w, "Greška pri premeštanju artikla", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/magacin?premesten=1", http.StatusSeeOther)
}

// ObrisiArtikal briše artikal po ID-u
func (h *Handler) ObrisiArtikal(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "artikal.obrisi"); !ok {
		return
	}
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Neispravan ID artikla", http.StatusBadRequest)
		return
	}

	if err := h.Artikli.Obrisi(r.Context(), id); err != nil {
		http.Error(w, "Greška pri brisanju artikla", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/magacin?obrisan=1", http.StatusSeeOther)
}

// PodaciMagacinskeKartice su podaci za karticu jednog artikla
type PodaciMagacinskeKartice struct {
	model.PodaciStranice
	Artikal  model.Artikal
	Promene  []model.MagacinskaPromenaSaDetaljem
}

// MagacinskaKartica prikazuje sve promene stanja za jedan artikal
func (h *Handler) MagacinskaKartica(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Neispravan ID artikla", http.StatusBadRequest)
		return
	}

	artikal, err := h.Artikli.DohvatiID(r.Context(), id)
	if err != nil {
		http.Error(w, "Artikal nije pronađen", http.StatusNotFound)
		return
	}

	promene, err := h.MagacinskePromeneRepo.Lista(r.Context(), &id, 0)
	if err != nil {
		http.Error(w, "Greška pri učitavanju promena", http.StatusInternalServerError)
		return
	}

	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "magacin"
	ps.NaslovStranice = "Kartica: " + artikal.Naziv

	h.renderujTemplate(w, "magacin_kartica", PodaciMagacinskeKartice{
		PodaciStranice: ps,
		Artikal:        *artikal,
		Promene:        promene,
	})
}
