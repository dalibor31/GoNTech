package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	appdb "ntech/internal/db"
	"ntech/internal/db/sqlite"
	"ntech/internal/middleware"
	"ntech/internal/model"

	"github.com/go-chi/chi/v5"
)

// PodaciKpo su podaci za pregled KPO knjige
type PodaciKpo struct {
	model.PodaciStranice
	Zapisi []model.KpoZapis
	Sume   model.KpoSume
	Od     string
	Do     string
}

// PodaciKpoForma su podaci za formu ručnog unosa u KPO
type PodaciKpoForma struct {
	model.PodaciStranice
	Danas string
}

// Kpo renderuje pregled knjige o ostvarenom prometu sa sumama za period.
func (h *Handler) Kpo(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "kpo.pregled"); !ok {
		return
	}
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	odStr := r.URL.Query().Get("od")
	doStr := r.URL.Query().Get("do")
	if odStr == "" || doStr == "" {
		sada := time.Now()
		prvi := time.Date(sada.Year(), sada.Month(), 1, 0, 0, 0, 0, sada.Location())
		poslednji := prvi.AddDate(0, 1, -1)
		if odStr == "" {
			odStr = prvi.Format("2006-01-02")
		}
		if doStr == "" {
			doStr = poslednji.Format("2006-01-02")
		}
	}

	zapisi, err := h.KpoRepo.Lista(r.Context(), parsiraDatumOpcionalno(odStr), parsiraDatumOpcionalno(doStr))
	if err != nil {
		http.Error(w, "Greška pri učitavanju KPO knjige", http.StatusInternalServerError)
		return
	}

	var sume model.KpoSume
	for _, z := range zapisi {
		sume.Prihod += z.Prihod
		sume.Broj++
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "kpo"
	ps.NaslovStranice = "KPO — knjiga o ostvarenom prometu"
	h.renderujTemplate(w, "kpo", PodaciKpo{
		PodaciStranice: ps,
		Zapisi:         zapisi,
		Sume:           sume,
		Od:             odStr,
		Do:             doStr,
	})
}

// NoviKpoUnos prikazuje formu za ručni unos u KPO.
func (h *Handler) NoviKpoUnos(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "kpo.dodaj"); !ok {
		return
	}
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}
	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "kpo"
	ps.NaslovStranice = "Novi KPO unos"
	h.renderujTemplate(w, "kpo_forma", PodaciKpoForma{
		PodaciStranice: ps,
		Danas:          time.Now().Format("2006-01-02"),
	})
}

// SacuvajKpoUnos prima POST i upisuje ručni KPO zapis.
func (h *Handler) SacuvajKpoUnos(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "kpo.dodaj"); !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	datum, err := time.Parse("2006-01-02", strings.TrimSpace(r.FormValue("datum_prometa")))
	if err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Datum prometa je obavezan.")
		http.Redirect(w, r, "/kpo/nova", http.StatusSeeOther)
		return
	}
	brojDok := strings.TrimSpace(r.FormValue("broj_dokumenta"))
	if brojDok == "" {
		middleware.SetFlash(w, r, h.DB, "greska", "Broj dokumenta je obavezan.")
		http.Redirect(w, r, "/kpo/nova", http.StatusSeeOther)
		return
	}

	z := model.KpoZapis{
		DatumPrometa:  datum,
		BrojDokumenta: brojDok,
		Opis:          strings.TrimSpace(r.FormValue("opis")),
		Prihod:        parsiraIznos(r.FormValue("prihod")),
		NacinPlacanja: strings.TrimSpace(r.FormValue("nacin_placanja")),
		Napomena:      strings.TrimSpace(r.FormValue("napomena")),
		Izvor:         "rucno",
	}

	if _, err := h.KpoRepo.Kreiraj(r.Context(), &z); err != nil {
		http.Error(w, "Greška pri čuvanju unosa", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/kpo?sacuvano=1", http.StatusSeeOther)
}

// ObrisiKpoUnos briše KPO zapis.
func (h *Handler) ObrisiKpoUnos(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "kpo.obrisi"); !ok {
		return
	}
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID", http.StatusBadRequest)
		return
	}
	if err := h.KpoRepo.Obrisi(r.Context(), id); err != nil {
		http.Error(w, "Greška pri brisanju unosa", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/kpo?obrisan=1", http.StatusSeeOther)
}

// KpoBackfill kreira KPO unose za sve prodaje i naplaćene servisne naloge koji ih nemaju.
func (h *Handler) KpoBackfill(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "kpo.dodaj"); !ok {
		return
	}
	ctx := r.Context()
	var kreirano, preskoceno int

	// prodajni nalozi
	nalozi, err := h.ProdajaRepo.Lista(ctx, appdb.ProdajaFilter{})
	if err != nil {
		http.Error(w, "Greška pri učitavanju prodaje", http.StatusInternalServerError)
		return
	}
	for _, nd := range nalozi {
		if nd.Stornirano {
			preskoceno++
			continue
		}
		postoji, _ := h.KpoRepo.PostojiZaIzvor(ctx, "prodaja", nd.ID)
		if postoji {
			preskoceno++
			continue
		}
		z := model.KpoZapis{
			DatumPrometa:  nd.Datum,
			BrojDokumenta: nd.BrojNaloga,
			Opis:          fmt.Sprintf("Prodaja %s", nd.BrojNaloga),
			Prihod:        nd.Ukupno,
			NacinPlacanja: nd.NacinPlacanja,
			Izvor:         "prodaja",
			IzvorID:       &nd.ID,
		}
		if _, e := h.KpoRepo.Kreiraj(ctx, &z); e != nil {
			slog.Error("kpo backfill: prodaja", "id", nd.ID, "error", e)
			continue
		}
		kreirano++
	}

	// naplaćeni servisni nalozi
	servisNalozi, err := h.ServisRepo.Lista(ctx, "", "")
	if err != nil {
		slog.Error("kpo backfill: greška pri učitavanju servisa", "error", err)
	} else {
		for _, sn := range servisNalozi {
			if sn.Status != model.StatusPreuzeto || sn.Naplaceno == 0 {
				preskoceno++
				continue
			}
			postoji, _ := h.KpoRepo.PostojiZaIzvor(ctx, "servis", sn.ID)
			if postoji {
				preskoceno++
				continue
			}
			datum := sn.DatumPrijema
			if sn.DatumZavrsetka != nil {
				datum = *sn.DatumZavrsetka
			}
			z := model.KpoZapis{
				DatumPrometa:  datum,
				BrojDokumenta: sn.BrojNaloga,
				Opis:          fmt.Sprintf("Servis %s", sn.BrojNaloga),
				Prihod:        sn.Naplaceno,
				NacinPlacanja: sn.NacinPlacanja,
				Izvor:         "servis",
				IzvorID:       &sn.ID,
			}
			if _, e := h.KpoRepo.Kreiraj(ctx, &z); e != nil {
				slog.Error("kpo backfill: servis", "id", sn.ID, "error", e)
				continue
			}
			kreirano++
		}
	}

	middleware.SetFlash(w, r, h.DB, "uspeh", fmt.Sprintf("Backfill završen: %d novih KPO unosa, %d preskočeno.", kreirano, preskoceno))
	http.Redirect(w, r, "/kpo", http.StatusSeeOther)
}
