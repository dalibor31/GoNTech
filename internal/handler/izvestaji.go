package handler

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"ntech/internal/db/sqlite"
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
