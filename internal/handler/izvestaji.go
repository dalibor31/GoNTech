package handler

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	appdbPkg "ntech/internal/db"
	"ntech/internal/db/sqlite"
	"ntech/internal/middleware"
	"ntech/internal/model"
)

var srpskaImenaMeseci = []string{
	"", "Januar", "Februar", "Mart", "April", "Maj", "Jun",
	"Jul", "Avgust", "Septembar", "Oktobar", "Novembar", "Decembar",
}

func formatujMesec(yyyymm string) string {
	var god, mes int
	if _, err := fmt.Sscanf(yyyymm, "%d-%d", &god, &mes); err != nil || mes < 1 || mes > 12 {
		return yyyymm
	}
	return fmt.Sprintf("%s %d", srpskaImenaMeseci[mes], god)
}

// PodaciIzvestaja su podaci za stranicu izveštaja
type PodaciIzvestaja struct {
	model.PodaciStranice
	MesecniPrihodi []MesecniPrihod
	GrafikonJSON   template.JS
	StariNalozi    []StariNalog
	TopArtikli     []TopArtikal
	TopKlijenti    []TopKlijent
}

// MesecniPrihod drži prihod od prodaje i servisa za jedan mesec
type MesecniPrihod struct {
	MesecPrikaz string
	Prodaja     float64
	Servis      float64
	Ukupno      float64
}

// StariNalog je servisni nalog bez datuma završetka stariji od 14 dana
type StariNalog struct {
	ID           int64
	BrojNaloga   string
	Uredjaj      string
	KlijentNaziv string
	Status       string
	DatumPrijema string
	DanaProslo   int
}

// TopArtikal je artikal rangiran po prodatoj količini
type TopArtikal struct {
	Rang           int
	Naziv          string
	Kategorija     string
	UkupnoKolicina int
	UkupnoPrihod   float64
}

// TopKlijent je klijent rangiran po ukupnoj vrednosti naloga
type TopKlijent struct {
	Rang           int
	Naziv          string
	BrojNaloga     int
	UkupnoVrednost float64
}

// Izvestaji renderuje stranicu sa izveštajima
func (h *Handler) Izvestaji(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "izvestaj.pregled"); !ok {
		return
	}
	ctx := r.Context()

	podesavanja, err := sqlite.DohvatiSvaPodesavanja(ctx, h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	// --- mesečni prihod: prodaja ---
	prodajaPoMesecu := map[string]float64{}
	if redovi, err := h.IzvestajRepo.MesecniPrihodProdaja(ctx); err != nil {
		slog.Error("izvestaji: prihod prodaja", "error", err)
	} else {
		for _, m := range redovi {
			prodajaPoMesecu[m.Mesec] = m.Iznos
		}
	}

	// --- mesečni prihod: servis ---
	servisPoMesecu := map[string]float64{}
	if redovi, err := h.IzvestajRepo.MesecniPrihodServis(ctx); err != nil {
		slog.Error("izvestaji: prihod servis", "error", err)
	} else {
		for _, m := range redovi {
			servisPoMesecu[m.Mesec] = m.Iznos
		}
	}

	// gradimo niz za poslednjih 12 meseci (hronološki)
	sada := time.Now()
	var mesecniPrihodi []MesecniPrihod
	var grafikonLabele []string
	var grafikonProdaja []float64
	var grafikonServis []float64

	for i := 11; i >= 0; i-- {
		t := sada.AddDate(0, -i, 0)
		kljuc := t.Format("2006-01")
		prod := prodajaPoMesecu[kljuc]
		serv := servisPoMesecu[kljuc]
		mesecniPrihodi = append(mesecniPrihodi, MesecniPrihod{
			MesecPrikaz: formatujMesec(kljuc),
			Prodaja:     prod,
			Servis:      serv,
			Ukupno:      prod + serv,
		})
		grafikonLabele = append(grafikonLabele, formatujMesec(kljuc))
		grafikonProdaja = append(grafikonProdaja, prod)
		grafikonServis = append(grafikonServis, serv)
	}

	type grafikonPodaci struct {
		Labele  []string  `json:"labele"`
		Prodaja []float64 `json:"prodaja"`
		Servis  []float64 `json:"servis"`
	}
	jsonBytes, _ := json.Marshal(grafikonPodaci{
		Labele:  grafikonLabele,
		Prodaja: grafikonProdaja,
		Servis:  grafikonServis,
	})

	// --- stari otvoreni nalozi (>14 dana bez završetka) ---
	var stariNalozi []StariNalog
	if redovi, err := h.IzvestajRepo.StariOtvoreniNalozi(ctx); err != nil {
		slog.Error("izvestaji: stari nalozi", "error", err)
	} else {
		for _, sn := range redovi {
			stariNalozi = append(stariNalozi, StariNalog{
				ID:           sn.ID,
				BrojNaloga:   sn.BrojNaloga,
				Uredjaj:      sn.Uredjaj,
				KlijentNaziv: sn.KlijentNaziv,
				Status:       sn.Status,
				DatumPrijema: sn.DatumPrijema.Format("02.01.2006."),
				DanaProslo:   int(time.Since(sn.DatumPrijema).Hours() / 24),
			})
		}
	}

	// --- top 10 najprodavanijih artikala ---
	var topArtikli []TopArtikal
	if redovi, err := h.IzvestajRepo.TopArtikli(ctx, 10); err != nil {
		slog.Error("izvestaji: top artikli", "error", err)
	} else {
		for i, a := range redovi {
			topArtikli = append(topArtikli, TopArtikal{
				Rang:           i + 1,
				Naziv:          a.Naziv,
				Kategorija:     a.Kategorija,
				UkupnoKolicina: a.UkupnoKolicina,
				UkupnoPrihod:   a.UkupnoPrihod,
			})
		}
	}

	// --- top 10 klijenata po ukupnoj vrednosti ---
	var topKlijenti []TopKlijent
	if redovi, err := h.IzvestajRepo.TopKlijenti(ctx, 10); err != nil {
		slog.Error("izvestaji: top klijenti", "error", err)
	} else {
		for i, k := range redovi {
			topKlijenti = append(topKlijenti, TopKlijent{
				Rang:           i + 1,
				Naziv:          k.Naziv,
				BrojNaloga:     k.BrojNaloga,
				UkupnoVrednost: k.UkupnoVrednost,
			})
		}
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "izvestaji"
	ps.NaslovStranice = "Izveštaji"

	podaci := PodaciIzvestaja{
		PodaciStranice: ps,
		MesecniPrihodi: mesecniPrihodi,
		GrafikonJSON:   template.JS(jsonBytes),
		StariNalozi:    stariNalozi,
		TopArtikli:     topArtikli,
		TopKlijenti:    topKlijenti,
	}

	h.renderujTemplate(w, "izvestaji", podaci)
}

// PodaciPrometногLista su podaci za prometni list magacina
type PodaciPrometногLista struct {
	model.PodaciStranice
	Promene []model.PrometniRed
	Od      string
	Do      string
	Ukupno  int
}

// PrometniListMagacina renderuje prometni list magacina za odabrani period
func (h *Handler) PrometniListMagacina(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "izvestaj.pregled"); !ok {
		return
	}

	danas := time.Now()
	odStr := r.URL.Query().Get("od")
	doStr := r.URL.Query().Get("do")

	if odStr == "" {
		odStr = danas.Format("2006-01-02")[:7] + "-01"
	}
	if doStr == "" {
		doStr = danas.Format("2006-01-02")
	}

	od, err := time.Parse("2006-01-02", odStr)
	if err != nil {
		od = time.Now()
	}
	do, err := time.Parse("2006-01-02", doStr)
	if err != nil {
		do = time.Now()
	}

	promene, err := h.IzvestajRepo.PrometniList(r.Context(), od, do)
	if err != nil {
		slog.Error("prometni list: greška", "error", err)
		promene = nil
	}

	podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "izvestaji"
	ps.NaslovStranice = "Prometni list"

	h.renderujTemplate(w, "prometni_list", PodaciPrometногLista{
		PodaciStranice: ps,
		Promene:        promene,
		Od:             odStr,
		Do:             doStr,
		Ukupno:         len(promene),
	})
}

// PodaciStanjaZaliha su podaci za izveštaj o stanju zaliha
type PodaciStanjaZaliha struct {
	model.PodaciStranice
	Zalihe         []model.StanjeZalihaRed
	UkupnaVrednost float64
	BrojArtikala   int
}

// StanjeZalihaIzvestaj renderuje izveštaj o trenutnom stanju zaliha
func (h *Handler) StanjeZalihaIzvestaj(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "izvestaj.pregled"); !ok {
		return
	}

	zalihe, err := h.IzvestajRepo.StanjeZaliha(r.Context())
	if err != nil {
		slog.Error("stanje zaliha: greška", "error", err)
		zalihe = nil
	}

	var ukupnaVrednost float64
	for _, z := range zalihe {
		ukupnaVrednost += z.VrednostZalihe
	}

	podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "izvestaji"
	ps.NaslovStranice = "Stanje zaliha"

	h.renderujTemplate(w, "stanje_zaliha", PodaciStanjaZaliha{
		PodaciStranice: ps,
		Zalihe:         zalihe,
		UkupnaVrednost: ukupnaVrednost,
		BrojArtikala:   len(zalihe),
	})
}

// PodaciPopisa su podaci za stranicu popisa
type PodaciPopisa struct {
	model.PodaciStranice
	Artikli  []model.ArtikalSaKategorijom
	Sacuvano bool
	Greska   string
}

// Popis prikazuje formu za unos stvarnog stanja (inventuru)
func (h *Handler) Popis(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "artikal.izmeni"); !ok {
		return
	}

	artikli, err := h.Artikli.Lista(r.Context(), appdbPkg.ArtikalFilter{})
	if err != nil {
		http.Error(w, "Greška pri učitavanju artikala", http.StatusInternalServerError)
		return
	}

	podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "izvestaji"
	ps.NaslovStranice = "Popis"

	h.renderujTemplate(w, "popis", PodaciPopisa{
		PodaciStranice: ps,
		Artikli:        artikli,
		Sacuvano:       r.URL.Query().Get("sacuvano") == "1",
	})
}

// SacuvajPopis prima POST formu sa prebrojenim količinama i upisuje korekcije
func (h *Handler) SacuvajPopis(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if !h.DozvoleRepo.ImaDozvolu(r.Context(), k.Uloga, "artikal.izmeni") {
		http.Error(w, "Nemate dozvolu za ovu akciju.", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	artikli, err := h.Artikli.Lista(r.Context(), appdbPkg.ArtikalFilter{})
	if err != nil {
		http.Error(w, "Greška pri učitavanju artikala", http.StatusInternalServerError)
		return
	}

	napomena := r.FormValue("napomena")
	if napomena == "" {
		napomena = "Godišnji popis"
	}

	var greskaBroj int
	for _, a := range artikli {
		kljuc := fmt.Sprintf("kolicina_%d", a.ID)
		vr := r.FormValue(kljuc)
		if vr == "" {
			continue
		}
		nova, err := strconv.Atoi(vr)
		if err != nil || nova < 0 {
			continue
		}
		if nova == a.Kolicina {
			continue
		}
		povecano := nova > a.Kolicina
		if err := h.Artikli.KorigujKolicinu(r.Context(), a.ID, nova, &k.ID, napomena); err != nil {
			slog.Error("popis: korekcija artikla", "id", a.ID, "error", err)
			greskaBroj++
			continue
		}
		// ako je stanje poraslo, počisti potraživane delove i otključaj naloge —
		// pokriveni delovi se odmah skidaju sa magacina (vidi ProveriIPocistiZaArtikal)
		if povecano {
			otkljucani, err := h.ServisniPotrazivaniDeloviRepo.ProveriIPocistiZaArtikal(r.Context(), a.ID)
			if err != nil {
				slog.Error("popis: provera potraživanih delova nije uspela", "artikal_id", a.ID, "error", err)
				continue
			}
			for _, nalogID := range otkljucani {
				if err := h.ServisRepo.AzurirajStatus(r.Context(), nalogID, model.StatusPrimljeno); err != nil {
					slog.Error("popis: automatski reset statusa naloga nije uspeo", "nalog_id", nalogID, "error", err)
				}
			}
		}
	}

	if greskaBroj > 0 {
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		ps := h.popuniPodaciStranice(r, podesavanja)
		ps.Stranica = "izvestaji"
		ps.NaslovStranice = "Popis"
		h.renderujTemplate(w, "popis", PodaciPopisa{
			PodaciStranice: ps,
			Artikli:        artikli,
			Greska:         fmt.Sprintf("Došlo je do greške pri upisivanju %d artikala.", greskaBroj),
		})
		return
	}

	http.Redirect(w, r, "/izvestaji/popis?sacuvano=1", http.StatusSeeOther)
}
