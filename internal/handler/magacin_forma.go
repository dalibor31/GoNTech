package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ntech/internal/db/sqlite"
	"ntech/internal/middleware"
	"ntech/internal/model"

	"github.com/go-chi/chi/v5"
)

// PodaciFormeArtikla su podaci za formu novog/izmenjenog artikla
type PodaciFormeArtikla struct {
	model.PodaciStranice
	Artikal            model.Artikal
	Kategorije         []model.Kategorija
	KategorijaIDStr    string
	Dobavljaci         []model.Dobavljac // svi dobavljači za izbor
	IzabraniDobavljaci map[int64]bool    // dobavljači vezani za artikal (za checked stanje)
	Greska             string
	Izmena             bool
}

// NoviArtikal prikazuje formu za unos novog artikla
func (h *Handler) NoviArtikal(w http.ResponseWriter, r *http.Request) {
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

	predlogSifre, err := h.Artikli.SledecaSifra(r.Context(), nil)
	if err != nil {
		slog.Error("greška pri generisanju predloga šifre", "err", err)
		predlogSifre = "ART-0001"
	}

	dobavljaci, _ := h.DobavljaciRepo.Lista(r.Context(), "")

	// tip se može predodabrati iz query-ja (dolazak sa stranice Usluge/Troškovi)
	tip := r.URL.Query().Get("tip")
	if tip != model.TipUsluga && tip != model.TipTrosak {
		tip = model.TipProizvod
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "magacin"
	ps.NaslovStranice = "Novi artikal"
	h.renderujFormuArtikla(w, PodaciFormeArtikla{
		PodaciStranice: ps,
		Kategorije:     kategorije,
		Dobavljaci:     dobavljaci,
		Artikal:        model.Artikal{Sifra: predlogSifre, Tip: tip, JedinicaMere: "kom", PdvStopa: h.podrazumevanaPdvStopa(r.Context())},
		Izmena:         false,
	})
}

// SacuvajArtikal prima POST formu i čuva novi artikal
func (h *Handler) SacuvajArtikal(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if !h.DozvoleRepo.ImaDozvolu(r.Context(), k.Uloga, "artikal.dodaj") {
		http.Error(w, "Nemate dozvolu za ovu akciju.", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	artikal, greska := parseFormuArtikla(r, h.podrazumevanaPdvStopa(r.Context()))
	if greska != "" {
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		kategorije, _ := h.KategorijeRepo.Lista(r.Context())
		dobavljaci, _ := h.DobavljaciRepo.Lista(r.Context(), "")
		katIDStr := ""
		if artikal.KategorijaID != nil {
			katIDStr = strconv.FormatInt(*artikal.KategorijaID, 10)
		}
		ps := h.popuniPodaciStranice(r, podesavanja)
		ps.Stranica = "magacin"
		ps.NaslovStranice = "Novi artikal"
		h.renderujFormuArtikla(w, PodaciFormeArtikla{
			PodaciStranice:     ps,
			Artikal:            artikal,
			Kategorije:         kategorije,
			KategorijaIDStr:    katIDStr,
			Dobavljaci:         dobavljaci,
			IzabraniDobavljaci: mapaDobavljaca(citajDobavljaceForme(r)),
			Greska:             greska,
			Izmena:             false,
		})
		return
	}

	// ako korisnik nije uneo šifru, auto-generišemo pre Kreiraj
	// (tako je dodela šifre atomična: ako INSERT padne na UNIQUE constraint,
	// Kreiraj vraća grešku umesto da šifra ostane NULL bez ikakve poruke)
	if artikal.Sifra == "" {
		autoSifra, e := h.Artikli.SledecaSifra(r.Context(), artikal.KategorijaID)
		if e != nil {
			autoSifra = fmt.Sprintf("ART-%04d", time.Now().UnixMilli()%10000)
		}
		artikal.Sifra = autoSifra
	}

	id, err := h.Artikli.Kreiraj(r.Context(), &artikal)
	if err != nil {
		http.Error(w, "Greška pri čuvanju artikla", http.StatusInternalServerError)
		return
	}
	artikal.ID = id

	// veži izabrane dobavljače (forma); modal nema to polje pa ostaje prazno
	if e := h.Artikli.PostaviDobavljaceArtikla(r.Context(), id, citajDobavljaceForme(r)); e != nil {
		slog.Error("čuvanje dobavljača artikla nije uspelo", "artikal_id", id, "error", e)
	}

	// fetch zahtev (iz modala) dobija JSON sa ID-em i nazivom novog artikla
	if r.Header.Get("X-Requested-With") == "fetch" {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":%d,"naziv":%q}`, id, artikal.Naziv)
		return
	}

	http.Redirect(w, r, "/magacin?sacuvano=1", http.StatusSeeOther)
}

// IzmeniArtikal prikazuje formu za izmenu postojećeg artikla
func (h *Handler) IzmeniArtikal(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Neispravan ID artikla", http.StatusBadRequest)
		return
	}

	artikal, err := h.Artikli.DohvatiID(r.Context(), id)
	if err != nil {
		http.Error(w, "Artikal nije pronađen", http.StatusNotFound)
		return
	}

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

	katIDStr := ""
	if artikal.KategorijaID != nil {
		katIDStr = strconv.FormatInt(*artikal.KategorijaID, 10)
	}

	dobavljaci, _ := h.DobavljaciRepo.Lista(r.Context(), "")
	izabrani := map[int64]bool{}
	if ids, e := h.Artikli.DobavljaciArtikla(r.Context(), id); e == nil {
		for _, did := range ids {
			izabrani[did] = true
		}
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "magacin"
	ps.NaslovStranice = "Izmeni artikal"
	h.renderujFormuArtikla(w, PodaciFormeArtikla{
		PodaciStranice:     ps,
		Artikal:            *artikal,
		Kategorije:         kategorije,
		KategorijaIDStr:    katIDStr,
		Dobavljaci:         dobavljaci,
		IzabraniDobavljaci: izabrani,
		Izmena:             true,
	})
}

// SacuvajIzmenuArtikla prima POST formu i čuva izmenu artikla
func (h *Handler) SacuvajIzmenuArtikla(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if !h.DozvoleRepo.ImaDozvolu(r.Context(), k.Uloga, "artikal.izmeni") {
		http.Error(w, "Nemate dozvolu za ovu akciju.", http.StatusForbidden)
		return
	}
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Neispravan ID artikla", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	artikal, greska := parseFormuArtikla(r, h.podrazumevanaPdvStopa(r.Context()))
	if greska != "" {
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		kategorije, _ := h.KategorijeRepo.Lista(r.Context())
		dobavljaci, _ := h.DobavljaciRepo.Lista(r.Context(), "")
		artikal.ID = id
		katIDStr := ""
		if artikal.KategorijaID != nil {
			katIDStr = strconv.FormatInt(*artikal.KategorijaID, 10)
		}
		ps := h.popuniPodaciStranice(r, podesavanja)
		ps.Stranica = "magacin"
		ps.NaslovStranice = "Izmeni artikal"
		h.renderujFormuArtikla(w, PodaciFormeArtikla{
			PodaciStranice:     ps,
			Artikal:            artikal,
			Kategorije:         kategorije,
			KategorijaIDStr:    katIDStr,
			Dobavljaci:         dobavljaci,
			IzabraniDobavljaci: mapaDobavljaca(citajDobavljaceForme(r)),
			Greska:             greska,
			Izmena:             true,
		})
		return
	}

	// stara prodajna cena i količina — za nivelacioni trag i hook potraživanih delova
	var staraCena float64
	var staraKolicina int
	if stari, e := h.Artikli.DohvatiID(r.Context(), id); e == nil {
		staraKolicina = stari.Kolicina
		staraCena = stari.ProdajnaCena
		// spreči promenu tipa sa proizvoda na uslugu/trošak ako artikal ima zalihu
		if stari.PratiLager() && stari.Kolicina > 0 && !artikal.PratiLager() {
			middleware.SetFlash(w, r, h.DB, "greska",
				fmt.Sprintf("Artikal ima %d %s na stanju. Prvo koriguj količinu na 0 pre promene tipa.", stari.Kolicina, stari.JedinicaMere))
			http.Redirect(w, r, "/magacin/izmeni/"+idStr, http.StatusSeeOther)
			return
		}
	}

	// promenu količine sprovodimo kroz KorigujKolicinu da bi se zabeležio magacinski
	// trag (korekcija); Izmeni menja samo metapodatke, ne dira stanje
	novaKolicina := artikal.Kolicina
	artikal.Kolicina = staraKolicina

	artikal.ID = id
	if err := h.Artikli.Izmeni(r.Context(), &artikal); err != nil {
		http.Error(w, "Greška pri čuvanju izmene", http.StatusInternalServerError)
		return
	}

	// ako se količina promenila, koriguj stanje uz magacinski trag
	if novaKolicina != staraKolicina {
		if e := h.Artikli.KorigujKolicinu(r.Context(), id, novaKolicina, &k.ID, "izmena artikla", model.PromenaKorekcija); e != nil {
			slog.Error("korekcija količine pri izmeni artikla nije uspela", "artikal_id", id, "error", e)
		}
	}

	// ažuriraj dobavljače artikla prema formi
	if e := h.Artikli.PostaviDobavljaceArtikla(r.Context(), id, citajDobavljaceForme(r)); e != nil {
		slog.Error("čuvanje dobavljača artikla nije uspelo", "artikal_id", id, "error", e)
	}

	// ako se prodajna cena promenila, automatski upiši nivelacioni zapis (izvor "izmena")
	if razlika := artikal.ProdajnaCena - staraCena; razlika > 0.005 || razlika < -0.005 {
		korisnikID := &k.ID
		if _, e := h.NivelacijaRepo.Kreiraj(r.Context(), &model.Nivelacija{
			ArtikalID:  id,
			StaraCena:  staraCena,
			NovaCena:   artikal.ProdajnaCena,
			Izvor:      "izmena",
			KorisnikID: korisnikID,
		}); e != nil {
			slog.Error("auto-nivelacija pri izmeni artikla nije upisana", "artikal_id", id, "error", e)
		}
	}

	// ako je stanje poraslo, proveri potraživane servisne delove
	if novaKolicina > staraKolicina {
		otkljucani, err := h.ServisniPotrazivaniDeloviRepo.ProveriIPocistiZaArtikal(r.Context(), id)
		if err != nil {
			slog.Error("provera potraživanih delova nije uspela", "artikal_id", id, "error", err)
		}
		for _, nalogID := range otkljucani {
			if err := h.ServisRepo.AzurirajStatus(r.Context(), nalogID, model.StatusPrimljeno); err != nil {
				slog.Error("automatski reset statusa naloga nije uspeo", "nalog_id", nalogID, "error", err)
			}
		}
	}

	http.Redirect(w, r, "/magacin?sacuvano=1", http.StatusSeeOther)
}

// mapaDobavljaca pretvara listu ID-jeva u mapu za checked stanje u formi
func mapaDobavljaca(ids []int64) map[int64]bool {
	m := make(map[int64]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

// citajDobavljaceForme čita izabrane dobavljače (checkbox dobavljaci[]) iz forme
func citajDobavljaceForme(r *http.Request) []int64 {
	var ids []int64
	for _, v := range r.Form["dobavljaci"] {
		if id, e := strconv.ParseInt(strings.TrimSpace(v), 10, 64); e == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

// parseFormuArtikla čita polja iz forme i vraća artikal i eventualnu grešku
// podrazumevanaStopa je opšta PDV stopa iz šifarnika (v. Handler.podrazumevanaPdvStopa) —
// koristi se samo kad korisnik nije uneo stopu, nikad hardkodovana vrednost.
func parseFormuArtikla(r *http.Request, podrazumevanaStopa float64) (model.Artikal, string) {
	naziv := r.FormValue("naziv")
	if naziv == "" {
		return model.Artikal{}, "Naziv artikla je obavezan."
	}

	var artikal model.Artikal
	artikal.Naziv = naziv
	artikal.Sifra = strings.TrimSpace(r.FormValue("sifra"))
	// prefiksi USL- i TRO- su rezervisani za šifre usluga i troškova
	if artikal.Sifra != "" {
		velika := strings.ToUpper(artikal.Sifra)
		if strings.HasPrefix(velika, "USL-") || strings.HasPrefix(velika, "TRO-") {
			return artikal, "Šifre sa prefiksom USL- i TRO- su rezervisane za usluge i troškove."
		}
	}
	artikal.Barkod = r.FormValue("barkod")
	artikal.Opis = r.FormValue("opis")
	artikal.Lokacija = r.FormValue("lokacija")
	artikal.Napomena = r.FormValue("napomena")

	// tip artikla — podrazumevano proizvod; usluga i trošak ne prate lager
	switch r.FormValue("tip") {
	case model.TipUsluga:
		artikal.Tip = model.TipUsluga
	case model.TipTrosak:
		artikal.Tip = model.TipTrosak
	default:
		artikal.Tip = model.TipProizvod
	}

	// jedinica mere — normalizacija: mala slova, samo slova/brojevi, max 4 karaktera
	artikal.JedinicaMere = normalizujJM(r.FormValue("jedinica_mere"))

	if k := r.FormValue("kolicina"); k != "" {
		v, err := strconv.Atoi(k)
		if err != nil || v < 0 {
			return artikal, "Količina mora biti pozitivan broj."
		}
		artikal.Kolicina = v
	}

	if k := r.FormValue("kolicina_min"); k != "" {
		v, err := strconv.Atoi(k)
		if err != nil || v < 0 {
			return artikal, "Minimalna količina mora biti pozitivan broj."
		}
		artikal.KolicinMin = v
	}

	// usluge i troškovi nemaju stanje na lageru
	if !artikal.PratiLager() {
		artikal.Kolicina = 0
		artikal.KolicinMin = 0
	}

	if c := r.FormValue("nabavna_cena"); c != "" {
		v, err := strconv.ParseFloat(c, 64)
		if err != nil || v < 0 {
			return artikal, "Nabavna cena mora biti pozitivan broj."
		}
		artikal.NabavnaCena = v
	}

	// PDV stopa — podrazumevano opšta stopa iz šifarnika
	pdvStopa := podrazumevanaStopa
	if p := r.FormValue("pdv_stopa"); p != "" {
		if v, err := strconv.ParseFloat(p, 64); err == nil && v >= 0 {
			pdvStopa = v
		}
	}
	artikal.PdvStopa = pdvStopa

	if c := r.FormValue("prodajna_cena"); c != "" {
		v, err := strconv.ParseFloat(c, 64)
		if err != nil || v < 0 {
			return artikal, "Prodajna cena mora biti pozitivan broj."
		}
		artikal.ProdajnaCena = v
	}

	// marža (%) je opciona; prazno polje ostaje NULL (artikal nasleđuje maržu kategorije/globalnu)
	if m := r.FormValue("marza"); m != "" {
		v, err := strconv.ParseFloat(m, 64)
		if err != nil || v < 0 {
			return artikal, "Marža mora biti pozitivan broj."
		}
		artikal.Marza = &v
	}

	if katID := r.FormValue("kategorija_id"); katID != "" {
		id, err := strconv.ParseInt(katID, 10, 64)
		if err == nil {
			artikal.KategorijaID = &id
		}
	}

	// kalkulacija cena_sa_pdv na osnovu unetog polja
	if c := r.FormValue("cena_sa_pdv"); c != "" {
		// korisnik je uneo bruto cenu → izračunaj neto
		if v, err := strconv.ParseFloat(c, 64); err == nil && v >= 0 {
			artikal.CenaSaPdv = v
			// ako prodajna_cena nije uneta ručno, izračunaj je
			if r.FormValue("prodajna_cena") == "" {
				artikal.ProdajnaCena = v / (1 + artikal.PdvStopa/100)
			}
		}
	} else {
		// korisnik je uneo neto cenu → izračunaj bruto
		artikal.CenaSaPdv = artikal.ProdajnaCena * (1 + artikal.PdvStopa/100)
	}

	return artikal, ""
}

// PredlogSifre vraća predlog auto-šifre za izabranu kategoriju (poziva forma pri promeni kategorije)
func (h *Handler) PredlogSifre(w http.ResponseWriter, r *http.Request) {
	var kategorijaID *int64
	if v := r.URL.Query().Get("kategorija"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			kategorijaID = &id
		}
	}
	sifra, err := h.Artikli.SledecaSifra(r.Context(), kategorijaID)
	if err != nil {
		http.Error(w, "Greška pri generisanju šifre", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(sifra))
}

// renderujFormuArtikla renderuje HTML formu za artikal
func (h *Handler) renderujFormuArtikla(w http.ResponseWriter, podaci PodaciFormeArtikla) {
	h.renderujTemplate(w, "magacin_forma", podaci)
}

// normalizujJM čisti jedinicu mere: mala slova, samo slova i brojevi, max 4 karaktera.
// Ako je rezultat prazan, vraća "kom" kao podrazumevanu vrednost.
func normalizujJM(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			if b.Len() >= 4 {
				break
			}
		}
	}
	if b.Len() == 0 {
		return "kom"
	}
	return b.String()
}
