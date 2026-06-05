package handler

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

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
	k := middleware.KorisnikIzKonteksta(r.Context())
	if k == nil || !h.DozvoleRepo.ImaDozvolu(r.Context(), k.Uloga, "izvestaj.pregled") {
		http.Error(w, "Nemate dozvolu za ovu akciju.", http.StatusForbidden)
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
	prodajaRed, err := h.DB.QueryContext(ctx, `
		SELECT substr(datum, 1, 7), SUM(ukupno)
		FROM prodajni_nalozi
		WHERE substr(datum, 1, 10) >= date('now', '-11 months', 'start of month')
		GROUP BY substr(datum, 1, 7)`)
	if err != nil {
		log.Printf("izvestaji: prihod prodaja: %v", err)
	} else {
		defer prodajaRed.Close()
		for prodajaRed.Next() {
			var mesec string
			var iznos float64
			if err := prodajaRed.Scan(&mesec, &iznos); err == nil {
				prodajaPoMesecu[mesec] = iznos
			}
		}
	}

	// --- mesečni prihod: servis ---
	servisPoMesecu := map[string]float64{}
	servisRed, err := h.DB.QueryContext(ctx, `
		SELECT substr(datum_zavrsetka, 1, 7), SUM(cena_konacna)
		FROM servisni_nalozi
		WHERE datum_zavrsetka IS NOT NULL
		  AND substr(datum_zavrsetka, 1, 10) >= date('now', '-11 months', 'start of month')
		GROUP BY substr(datum_zavrsetka, 1, 7)`)
	if err != nil {
		log.Printf("izvestaji: prihod servis: %v", err)
	} else {
		defer servisRed.Close()
		for servisRed.Next() {
			var mesec string
			var iznos float64
			if err := servisRed.Scan(&mesec, &iznos); err == nil {
				servisPoMesecu[mesec] = iznos
			}
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
	stariRed, err := h.DB.QueryContext(ctx, `
		SELECT sn.id, sn.broj_naloga, sn.uredjaj, sn.status, sn.datum_prijema,
		       COALESCE(NULLIF(k.naziv_firme, ''), TRIM(COALESCE(k.ime, '') || ' ' || COALESCE(k.prezime, '')), '—') AS klijent_naziv
		FROM servisni_nalozi sn
		LEFT JOIN klijenti k ON k.id = sn.klijent_id
		WHERE sn.datum_zavrsetka IS NULL
		  AND substr(sn.datum_prijema, 1, 10) <= date('now', '-14 days')
		ORDER BY sn.datum_prijema ASC`)

	var stariNalozi []StariNalog
	if err != nil {
		log.Printf("izvestaji: stari nalozi: %v", err)
	} else {
		defer stariRed.Close()
		for stariRed.Next() {
			var sn StariNalog
			var datumVreme time.Time
			if err := stariRed.Scan(&sn.ID, &sn.BrojNaloga, &sn.Uredjaj, &sn.Status, &datumVreme, &sn.KlijentNaziv); err == nil {
				sn.DatumPrijema = datumVreme.Format("02.01.2006.")
				sn.DanaProslo = int(time.Since(datumVreme).Hours() / 24)
				stariNalozi = append(stariNalozi, sn)
			}
		}
	}

	// --- top 10 najprodavanijih artikala ---
	artRed, err := h.DB.QueryContext(ctx, `
		SELECT a.naziv, COALESCE(k.naziv, '—'), SUM(sp.kolicina), SUM(sp.ukupno)
		FROM stavke_prodaje sp
		JOIN artikli a ON a.id = sp.artikal_id
		LEFT JOIN kategorije k ON k.id = a.kategorija_id
		GROUP BY sp.artikal_id
		ORDER BY SUM(sp.kolicina) DESC
		LIMIT 10`)

	var topArtikli []TopArtikal
	if err != nil {
		log.Printf("izvestaji: top artikli: %v", err)
	} else {
		defer artRed.Close()
		for artRed.Next() {
			var a TopArtikal
			if err := artRed.Scan(&a.Naziv, &a.Kategorija, &a.UkupnoKolicina, &a.UkupnoPrihod); err == nil {
				a.Rang = len(topArtikli) + 1
				topArtikli = append(topArtikli, a)
			}
		}
	}

	// --- top 10 klijenata po ukupnoj vrednosti ---
	klijRed, err := h.DB.QueryContext(ctx, `
		SELECT
		    COALESCE(NULLIF(k.naziv_firme, ''), TRIM(COALESCE(k.ime, '') || ' ' || COALESCE(k.prezime, ''))) AS naziv,
		    COALESCE(p.ukupno_prodaja, 0) + COALESCE(s.ukupno_servis, 0) AS ukupno_vrednost,
		    COALESCE(p.broj_prodaja, 0) + COALESCE(s.broj_servisa, 0) AS broj_naloga
		FROM klijenti k
		LEFT JOIN (
		    SELECT klijent_id, SUM(ukupno) AS ukupno_prodaja, COUNT(*) AS broj_prodaja
		    FROM prodajni_nalozi GROUP BY klijent_id
		) p ON p.klijent_id = k.id
		LEFT JOIN (
		    SELECT klijent_id, SUM(cena_konacna) AS ukupno_servis, COUNT(*) AS broj_servisa
		    FROM servisni_nalozi WHERE cena_konacna IS NOT NULL GROUP BY klijent_id
		) s ON s.klijent_id = k.id
		WHERE COALESCE(p.ukupno_prodaja, 0) + COALESCE(s.ukupno_servis, 0) > 0
		ORDER BY ukupno_vrednost DESC
		LIMIT 10`)

	var topKlijenti []TopKlijent
	if err != nil {
		log.Printf("izvestaji: top klijenti: %v", err)
	} else {
		defer klijRed.Close()
		for klijRed.Next() {
			var k TopKlijent
			if err := klijRed.Scan(&k.Naziv, &k.UkupnoVrednost, &k.BrojNaloga); err == nil {
				k.Rang = len(topKlijenti) + 1
				topKlijenti = append(topKlijenti, k)
			}
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
