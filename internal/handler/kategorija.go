package handler

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
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

// PromenaSifre je jedna stavka plana usklađivanja šifri — artikal čija
// trenutna šifra ne odgovara prefiksu njegove kategorije, sa predloženom
// novom šifrom.
type PromenaSifre struct {
	ArtikalID       int64
	Naziv           string
	KategorijaNaziv string
	StaraSifra      string
	NovaSifra       string
}

// izracunajPlanUskladjivanjaSifri prolazi kroz sve artikle (po ID-u, redosled
// unosa) i za svaki proverava da li šifra odgovara PREFIKS-NNNN obrascu
// njegove kategorije (kod kategorije, ili "ART" ako artikal nema kategoriju).
// Artikli iz kategorija bez postavljenog koda se preskaču. Novi broj je prvi
// slobodan (počev od 0001) za taj prefiks — uzima u obzir i šifre koje već
// postoje kod drugih artikala i one dodeljene ranije u istom prolazu.
func izracunajPlanUskladjivanjaSifri(artikli []model.ArtikalSaKategorijom, kategorije []model.Kategorija) []PromenaSifre {
	// Lista artikala dolazi poređana po nazivu — kandidati se obrađuju po ID-u
	// (redosled unosa), pa se pravi sopstvena poređana kopija.
	artikli = append([]model.ArtikalSaKategorijom(nil), artikli...)
	sort.Slice(artikli, func(i, j int) bool { return artikli[i].ID < artikli[j].ID })

	kodPoKategoriji := make(map[int64]string, len(kategorije))
	for _, k := range kategorije {
		kodPoKategoriji[k.ID] = k.Kod
	}

	// zauzeti[prefiks] = skup već iskorišćenih brojeva (iz postojećih šifri svih artikala)
	zauzeti := make(map[string]map[int]bool)
	zauzmi := func(prefiks string, broj int) {
		if zauzeti[prefiks] == nil {
			zauzeti[prefiks] = make(map[int]bool)
		}
		zauzeti[prefiks][broj] = true
	}
	parsirajBroj := func(sifra, prefiks string) (int, bool) {
		rep := prefiks + "-"
		if !strings.HasPrefix(sifra, rep) {
			return 0, false
		}
		n, err := strconv.Atoi(sifra[len(rep):])
		if err != nil || n < 1 {
			return 0, false
		}
		return n, true
	}

	prvoSlobodan := func(prefiks string) int {
		broj := 1
		for zauzeti[prefiks][broj] {
			broj++
		}
		return broj
	}

	// popiši postojeće brojeve po prefiksu iz svih trenutnih šifri (oblika PREFIKS-NNNN)
	for _, a := range artikli {
		if a.Sifra == "" {
			continue
		}
		i := strings.LastIndex(a.Sifra, "-")
		if i <= 0 {
			continue
		}
		prefiks, rep := a.Sifra[:i], a.Sifra[i+1:]
		if n, err := strconv.Atoi(rep); err == nil && n >= 1 {
			zauzmi(prefiks, n)
		}
	}

	var plan []PromenaSifre
	for _, a := range artikli {
		ocekivaniPrefiks := "ART"
		if a.KategorijaID != nil {
			kod, ima := kodPoKategoriji[*a.KategorijaID]
			if !ima || kod == "" {
				continue // kategorija bez koda — preskoči
			}
			ocekivaniPrefiks = kod
		}

		if _, tacna := parsirajBroj(a.Sifra, ocekivaniPrefiks); tacna {
			continue // šifra već odgovara obrascu za ovu kategoriju
		}

		broj := prvoSlobodan(ocekivaniPrefiks)
		zauzmi(ocekivaniPrefiks, broj)
		novaSifra := fmt.Sprintf("%s-%04d", ocekivaniPrefiks, broj)

		plan = append(plan, PromenaSifre{
			ArtikalID:       a.ID,
			Naziv:           a.Naziv,
			KategorijaNaziv: a.KategorijaNaziv,
			StaraSifra:      a.Sifra,
			NovaSifra:       novaSifra,
		})
	}

	return plan
}

// PodaciUskladjivanjaSifri su podaci za stranicu pregleda usklađivanja šifri
type PodaciUskladjivanjaSifri struct {
	model.PodaciStranice
	Plan []PromenaSifre
}

// PregledUskladjivanjaSifri prikazuje spisak artikala čija šifra ne odgovara
// prefiksu njihove kategorije, sa predlogom nove šifre — pre potvrde.
func (h *Handler) PregledUskladjivanjaSifri(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "artikal.izmeni"); !ok {
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
	kategorije, err := h.KategorijeRepo.Lista(r.Context())
	if err != nil {
		http.Error(w, "Greška pri učitavanju kategorija", http.StatusInternalServerError)
		return
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "magacin"
	ps.NaslovStranice = "Usklađivanje šifri"

	h.renderujTemplate(w, "kategorije_uskladi_sifre", PodaciUskladjivanjaSifri{
		PodaciStranice: ps,
		Plan:           izracunajPlanUskladjivanjaSifri(artikli, kategorije),
	})
}

// PotvrdiUskladjivanjeSifri prima POST sa tačnim spiskom promena prikazanim
// na stranici pregleda (artikal_id[] / nova_sifra[]) i upisuje ih. Ne računa
// plan iznova — primenjuje tačno ono što je pregledano i potvrđeno.
func (h *Handler) PotvrdiUskladjivanjeSifri(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "artikal.izmeni"); !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	idovi := r.Form["artikal_id"]
	sifre := r.Form["nova_sifra"]

	broj := 0
	for i := 0; i < len(idovi) && i < len(sifre); i++ {
		id, err := strconv.ParseInt(idovi[i], 10, 64)
		if err != nil {
			continue
		}
		if err := h.Artikli.AzurirajSifru(r.Context(), id, sifre[i]); err != nil {
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
		if errors.Is(err, db.ErrKategorijaDuplikat) {
			middleware.SetFlash(w, r, h.DB, "greska", "Kategorija sa tim nazivom ili kodom već postoji.")
			http.Redirect(w, r, "/magacin/kategorije", http.StatusSeeOther)
			return
		}
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
		if errors.Is(err, db.ErrKategorijaDuplikat) {
			middleware.SetFlash(w, r, h.DB, "greska", "Kategorija sa tim nazivom ili kodom već postoji.")
			http.Redirect(w, r, "/magacin/kategorije", http.StatusSeeOther)
			return
		}
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
