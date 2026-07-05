package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"ntech/internal/db"
	"ntech/internal/db/sqlite"
	"ntech/internal/middleware"
	"ntech/internal/model"

	"github.com/go-chi/chi/v5"
)

// PodaciKategorija su podaci za stranicu kategorija
type PodaciKategorija struct {
	model.PodaciStranice
	Kategorije      []model.Kategorija
	Sacuvano        bool
	Obrisana        bool
	SifreDodeljene  int
	PrikaziSifreDod bool
}

// Kategorije renderuje listu kategorija
func (h *Handler) Kategorije(w http.ResponseWriter, r *http.Request) {
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	kategorije, err := h.KategorijeRepo.Lista(r.Context())
	if err != nil {
		http.Error(w, "Greška pri učitavanju kategorija", http.StatusInternalServerError)
		return
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "magacin"
	ps.NaslovStranice = "Kategorije"
	sifreDodeljeneStr := r.URL.Query().Get("sifre_dodeljene")
	sifreDodeljene, _ := strconv.Atoi(sifreDodeljeneStr)
	podaci := PodaciKategorija{
		PodaciStranice:  ps,
		Kategorije:      kategorije,
		Sacuvano:        r.URL.Query().Get("sacuvano") == "1",
		Obrisana:        r.URL.Query().Get("obrisana") == "1",
		SifreDodeljene:  sifreDodeljene,
		PrikaziSifreDod: sifreDodeljeneStr != "",
	}

	h.renderujTemplate(w, "kategorije", podaci)
}

// DodeliSifreArtiklima prolazi kroz sve artikle bez šifre i dodeljuje im
// šifru na osnovu prefiksa njihove kategorije (isti generator kao za nove
// artikle). Radi sekvencijalno — jedan artikal se upiše pre nego što se za
// sledeći izračuna predlog — da artikli iz iste kategorije ne bi dobili isti broj.
func (h *Handler) DodeliSifreArtiklima(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "artikal.izmeni"); !ok {
		return
	}

	artikli, err := h.Artikli.Lista(r.Context(), db.ArtikalFilter{})
	if err != nil {
		http.Error(w, "Greška pri učitavanju artikala", http.StatusInternalServerError)
		return
	}

	broj := 0
	for _, a := range artikli {
		if a.Sifra != "" {
			continue
		}
		sifra, err := h.Artikli.SledecaSifra(r.Context(), a.KategorijaID)
		if err != nil {
			continue
		}
		if err := h.Artikli.AzurirajSifru(r.Context(), a.ID, sifra); err != nil {
			continue
		}
		broj++
	}

	http.Redirect(w, r, "/magacin/kategorije?sacuvano=1&sifre_dodeljene="+strconv.Itoa(broj), http.StatusSeeOther)
}

// DodajKategoriju prima POST i čuva novu kategoriju
func (h *Handler) DodajKategoriju(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "kategorija.dodaj"); !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	naziv := r.FormValue("naziv")
	if naziv == "" {
		http.Redirect(w, r, "/magacin/kategorije", http.StatusSeeOther)
		return
	}

	k := &model.Kategorija{
		Naziv: naziv,
		Opis:  r.FormValue("opis"),
		Kod:   normalizujKod(r.FormValue("kod")),
		Marza: parsirajMarzu(r.FormValue("marza")),
	}

	if _, err := h.KategorijeRepo.Kreiraj(r.Context(), k); err != nil {
		http.Error(w, "Greška pri čuvanju kategorije", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/magacin/kategorije?sacuvano=1", http.StatusSeeOther)
}

// IzmeniKategoriju prima POST i ažurira naziv, opis i maržu postojeće kategorije
func (h *Handler) IzmeniKategoriju(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "kategorija.izmeni"); !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Neispravan ID kategorije", http.StatusBadRequest)
		return
	}

	naziv := r.FormValue("naziv")
	if naziv == "" {
		http.Redirect(w, r, "/magacin/kategorije", http.StatusSeeOther)
		return
	}

	k := &model.Kategorija{
		ID:    id,
		Naziv: naziv,
		Opis:  r.FormValue("opis"),
		Kod:   normalizujKod(r.FormValue("kod")),
		Marza: parsirajMarzu(r.FormValue("marza")),
	}

	if err := h.KategorijeRepo.Izmeni(r.Context(), k); err != nil {
		http.Error(w, "Greška pri čuvanju izmene kategorije", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/magacin/kategorije?sacuvano=1", http.StatusSeeOther)
}

// parsirajMarzu pretvara tekst iz forme u *float64; prazno/neispravno → nil (NULL u bazi)
func parsirajMarzu(s string) *float64 {
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

// normalizujKod čisti kôd kategorije za upotrebu kao prefiks šifre:
// velika slova, zadržava samo slova i brojeve (bez razmaka i specijalnih znakova).
func normalizujKod(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	kod := b.String()
	// USL i TRO su rezervisani prefiksi (usluge i troškovi) — ne dozvoljavamo ih
	// kao kôd kategorije da auto-šifra artikla ne bi pala u rezervisani prostor
	if kod == "USL" || kod == "TRO" {
		return ""
	}
	return kod
}

// ObrisiKategoriju briše kategoriju po ID-u
func (h *Handler) ObrisiKategoriju(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "kategorija.obrisi"); !ok {
		return
	}
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Neispravan ID kategorije", http.StatusBadRequest)
		return
	}

	if err := h.KategorijeRepo.Obrisi(r.Context(), id); err != nil {
		if errors.Is(err, db.ErrKategorijaUUpotrebi) {
			middleware.SetFlash(w, r, h.DB, "greska", "Kategorija je u upotrebi kod artikala i ne može se obrisati.")
			http.Redirect(w, r, "/magacin/kategorije", http.StatusSeeOther)
			return
		}
		http.Error(w, "Greška pri brisanju kategorije", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/magacin/kategorije?obrisana=1", http.StatusSeeOther)
}
