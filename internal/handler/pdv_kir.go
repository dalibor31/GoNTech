package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	appdb "ntech/internal/db"
	"ntech/internal/db/sqlite"
	"ntech/internal/middleware"
	"ntech/internal/model"

	"math"

	"github.com/go-chi/chi/v5"
)

// PodaciPdvKir su podaci za pregled knjige izdatih računa
type PodaciPdvKir struct {
	model.PodaciStranice
	Zapisi            []model.PdvKir
	Sume              model.PdvKirSume
	Od                string
	Do                string
	Zastareo          map[int64]bool
	SumnjiviDuplikati []model.ProdajniNalogSaDetaljem // B2B prodaje preskočene pri backfill-u (double-submit)
}

// PodaciPdvKirForma su podaci za formu unosa zapisa KIR
type PodaciPdvKirForma struct {
	model.PodaciStranice
	Greska   string
	Danas    string          // podrazumevani datum u formi
	Klijenti []model.Klijent // za izbor kupca iz postojećih klijenata
}

// parsiraDatumOpcionalno vraća datum iz YYYY-MM-DD; prazan string daje nulti datum (bez filtera)
func parsiraDatumOpcionalno(s string) time.Time {
	t, err := time.Parse("2006-01-02", strings.TrimSpace(s))
	if err != nil {
		return time.Time{}
	}
	return t
}

// parsiraIznos čita decimalni broj iz forme (prihvata i zarez); prazno/neispravno daje 0
func parsiraIznos(s string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(strings.Replace(s, ",", ".", 1)), 64)
	if err != nil || v < 0 {
		return 0
	}
	return v
}

// PdvKir renderuje pregled knjige izdatih računa sa sumama po stopama
func (h *Handler) PdvKir(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "pdv.pregled"); !ok {
		return
	}
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	odStr := r.URL.Query().Get("od")
	doStr := r.URL.Query().Get("do")
	// podrazumevano: tekući mesec (od prvog do poslednjeg dana)
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
	zapisi, err := h.PdvKirRepo.Lista(r.Context(), parsiraDatumOpcionalno(odStr), parsiraDatumOpcionalno(doStr))
	if err != nil {
		http.Error(w, "Greška pri učitavanju knjige izdatih računa", http.StatusInternalServerError)
		return
	}

	// zbirni unos dnevnog pazara nema povratnu vezu ka pojedinačnim nalozima —
	// ako je posle upisa neki maloprodajni nalog tog dana storniran ili dodat,
	// upisani iznos ostaje zastareo dok korisnik svesno ne osveži tu formu
	zastareo := map[int64]bool{}
	for _, z := range zapisi {
		if z.Izvor != "rucno" || !strings.HasPrefix(z.BrojDokumenta, "FISK-") {
			continue
		}
		promet, err := h.ProdajaRepo.DnevniPrometMaloprodaje(r.Context(), z.DatumPrometa.Format("2006-01-02"))
		if err != nil {
			continue
		}
		trenutno := math.Round((promet.OsnovicaOpsta+promet.PdvOpsta+promet.OsnovicaPosebna+promet.PdvPosebna)*100) / 100
		if math.Abs(trenutno-z.Ukupno) > 0.005 {
			zastareo[z.ID] = true
		}
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "pdv-kir"
	ps.NaslovStranice = "KIR — knjiga izdatih računa"

	// detektuj sumnjive duplikate (double-submit) da ih korisnik vidi
	var sumnjivi []model.ProdajniNalogSaDetaljem
	if _, _, kandidati, err := h.kirKandidatiProdaje(r.Context()); err == nil {
		duplikati := sumnjiviDuplikati(kandidati)
		if len(duplikati) > 0 {
			nalozi, _ := h.ProdajaRepo.Lista(r.Context(), appdb.ProdajaFilter{})
			for _, nd := range nalozi {
				if duplikati[nd.ID] {
					sumnjivi = append(sumnjivi, nd)
				}
			}
		}
	}

	h.renderujTemplate(w, "pdv_kir", PodaciPdvKir{
		PodaciStranice:    ps,
		Zapisi:            zapisi,
		Sume:              model.SumirajKir(zapisi),
		Od:                odStr,
		Do:                doStr,
		Zastareo:          zastareo,
		SumnjiviDuplikati: sumnjivi,
	})
}

// NoviPdvKir prikazuje praznu formu za unos zapisa u KIR
func (h *Handler) NoviPdvKir(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "pdv.dodaj"); !ok {
		return
	}
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}
	// klijenti za izbor kupca; greška se ne prekida (forma radi i sa ručnim unosom)
	klijenti, _ := h.KlijentiRepo.Lista(r.Context(), "")

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "pdv-kir"
	ps.NaslovStranice = "Novi izlazni račun (KIR)"
	h.renderujTemplate(w, "pdv_kir_forma", PodaciPdvKirForma{
		PodaciStranice: ps,
		Danas:          time.Now().Format("2006-01-02"),
		Klijenti:       klijenti,
	})
}

// SacuvajPdvKir prima POST i upisuje novi zapis u KIR
func (h *Handler) SacuvajPdvKir(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "pdv.dodaj"); !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	datumPrometa, e1 := time.Parse("2006-01-02", strings.TrimSpace(r.FormValue("datum_prometa")))
	datumKnjizenja, e2 := time.Parse("2006-01-02", strings.TrimSpace(r.FormValue("datum_knjizenja")))
	brojDokumenta := strings.TrimSpace(r.FormValue("broj_dokumenta"))
	kupacNaziv := strings.TrimSpace(r.FormValue("kupac_naziv"))

	greska := ""
	switch {
	case e1 != nil:
		greska = "Datum prometa je obavezan i mora biti ispravan."
	case e2 != nil:
		greska = "Datum knjiženja je obavezan i mora biti ispravan."
	case brojDokumenta == "":
		greska = "Broj dokumenta je obavezan."
	case kupacNaziv == "":
		greska = "Naziv kupca je obavezan."
	}
	if greska != "" {
		middleware.SetFlash(w, r, h.DB, "greska", greska)
		http.Redirect(w, r, "/pdv/kir/nova", http.StatusSeeOther)
		return
	}

	z := model.PdvKir{
		DatumPrometa:      datumPrometa,
		DatumKnjizenja:    datumKnjizenja,
		BrojDokumenta:     brojDokumenta,
		KupacNaziv:        kupacNaziv,
		KupacPib:          strings.TrimSpace(r.FormValue("kupac_pib")),
		KupacMesto:        strings.TrimSpace(r.FormValue("kupac_mesto")),
		OsnovicaOpsta:     parsiraIznos(r.FormValue("osnovica_opsta")),
		PdvOpsta:          parsiraIznos(r.FormValue("pdv_opsta")),
		OsnovicaPosebna:   parsiraIznos(r.FormValue("osnovica_posebna")),
		PdvPosebna:        parsiraIznos(r.FormValue("pdv_posebna")),
		OslobodenSaPravom: parsiraIznos(r.FormValue("osloboden_sa_pravom")),
		OslobodenBezPrava: parsiraIznos(r.FormValue("osloboden_bez_prava")),
		Napomena:          strings.TrimSpace(r.FormValue("napomena")),
	}
	// ukupna naknada sa PDV — zbir svih osnovica, PDV-a i oslobođenog prometa (računa server)
	z.Ukupno = z.OsnovicaOpsta + z.PdvOpsta + z.OsnovicaPosebna + z.PdvPosebna +
		z.OslobodenSaPravom + z.OslobodenBezPrava

	if _, err := h.PdvKirRepo.Kreiraj(r.Context(), &z); err != nil {
		http.Error(w, "Greška pri čuvanju zapisa", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/pdv/kir?sacuvano=1", http.StatusSeeOther)
}

// ObrisiPdvKir briše zapis iz KIR
func (h *Handler) ObrisiPdvKir(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "pdv.obrisi"); !ok {
		return
	}
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID zapisa", http.StatusBadRequest)
		return
	}
	if err := h.PdvKirRepo.Obrisi(r.Context(), id); err != nil {
		http.Error(w, "Greška pri brisanju zapisa", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/pdv/kir?obrisan=1", http.StatusSeeOther)
}

// KirBackfillProdaje kreira KIR zapise za sve B2B prodaje koje ih nemaju.
// Namenjen za jednokratno popunjavanje starih podataka.
func (h *Handler) KirBackfillProdaje(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "pdv.dodaj"); !ok {
		return
	}
	ctx := r.Context()
	sada := time.Now()

	nalozi, postojiMap, kandidati, err := h.kirKandidatiProdaje(ctx)
	if err != nil {
		http.Error(w, "Greška pri učitavanju prodaje", http.StatusInternalServerError)
		return
	}
	duplikati := sumnjiviDuplikati(kandidati)

	var kreirano, preskoceno, sumnjivo int
	for _, nd := range nalozi {
		if nd.KlijentID == nil {
			preskoceno++
			continue
		}
		if postojiMap[nd.ID] {
			preskoceno++
			continue
		}
		if duplikati[nd.ID] {
			sumnjivo++
			continue
		}
		stavke, err := h.ProdajaRepo.DohvatiStavke(ctx, nd.ID)
		if err != nil {
			slog.Error("backfill KIR: greška pri čitanju stavki", "prodaja_id", nd.ID, "error", err)
			continue
		}
		var ss []model.StavkaProdaje
		for _, s := range stavke {
			ss = append(ss, s.StavkaProdaje)
		}
		klijent, err := h.KlijentiRepo.DohvatiID(ctx, *nd.KlijentID)
		if err != nil {
			slog.Error("backfill KIR: greška pri čitanju klijenta", "prodaja_id", nd.ID, "error", err)
			continue
		}
		pib := klijent.PIB
		if klijent.Tip != "pravno" {
			pib = klijent.JMBG
		}
		// backfill upisuje stare naloge sa zakašnjenjem — datum_knjizenja je danas, ne datum prodaje
		kir := model.KirIzProdaje(nd.ProdajniNalog, ss, klijent.PunoIme(), pib, klijent.Mesto, sada)
		if _, e := h.PdvKirRepo.Kreiraj(ctx, &kir); e != nil {
			slog.Error("backfill KIR: greška pri kreiranju zapisa", "prodaja_id", nd.ID, "error", e)
			continue
		}
		kreirano++
	}

	poruka := fmt.Sprintf("Backfill završen: %d novih KIR zapisa, %d preskočeno.", kreirano, preskoceno)
	if sumnjivo > 0 {
		poruka += fmt.Sprintf(" %d sumnjivih duplikata NIJE upisano — proveri ručno.", sumnjivo)
	}
	middleware.SetFlash(w, r, h.DB, "uspeh", poruka)
	http.Redirect(w, r, "/pdv/kir", http.StatusSeeOther)
}

// KirBackfillServisa kreira KIR zapise za sve preuzete servisne naloge sa
// identifikovanim kupcem koji ih nemaju. Namenjen za jednokratno popunjavanje.
func (h *Handler) KirBackfillServisa(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "pdv.dodaj"); !ok {
		return
	}
	ctx := r.Context()
	sada := time.Now()

	nalozi, postojiMap, kandidati, err := h.kirKandidatiServisa(ctx)
	if err != nil {
		http.Error(w, "Greška pri učitavanju servisa", http.StatusInternalServerError)
		return
	}
	duplikati := sumnjiviDuplikati(kandidati)

	var kreirano, preskoceno, sumnjivo int
	for _, sn := range nalozi {
		if postojiMap[sn.ID] {
			preskoceno++
			continue
		}
		if duplikati[sn.ID] {
			sumnjivo++
			continue
		}
		radovi, err := h.ServisniRadoviRepo.DohvatiZaNalog(ctx, sn.ID)
		if err != nil {
			slog.Error("backfill KIR: greška pri čitanju radova", "servis_id", sn.ID, "error", err)
			continue
		}
		delovi, err := h.ServisniDeloviRepo.DohvatiZaNalog(ctx, sn.ID)
		if err != nil {
			slog.Error("backfill KIR: greška pri čitanju delova", "servis_id", sn.ID, "error", err)
			continue
		}
		klijent, err := h.KlijentiRepo.DohvatiID(ctx, *sn.KlijentID)
		if err != nil {
			slog.Error("backfill KIR: greška pri čitanju klijenta", "servis_id", sn.ID, "error", err)
			continue
		}
		pib := klijent.PIB
		if klijent.Tip != "pravno" {
			pib = klijent.JMBG
		}
		datumPrometa := sn.DatumPrijema
		if sn.DatumZavrsetka != nil {
			datumPrometa = *sn.DatumZavrsetka
		}
		kir := model.KirIzServisa(sn.ServisniNalog, radovi, delovi, klijent.PunoIme(), pib, klijent.Mesto, datumPrometa, sada)
		if kir.Ukupno <= 0 {
			preskoceno++
			continue
		}
		if _, e := h.PdvKirRepo.Kreiraj(ctx, &kir); e != nil {
			slog.Error("backfill KIR: greška pri kreiranju zapisa", "servis_id", sn.ID, "error", e)
			continue
		}
		kreirano++
	}

	poruka := fmt.Sprintf("Backfill završen: %d novih KIR zapisa, %d preskočeno.", kreirano, preskoceno)
	if sumnjivo > 0 {
		poruka += fmt.Sprintf(" %d sumnjivih duplikata NIJE upisano — proveri ručno.", sumnjivo)
	}
	middleware.SetFlash(w, r, h.DB, "uspeh", poruka)
	http.Redirect(w, r, "/pdv/kir", http.StatusSeeOther)
}

// PodaciDnevniPazar su podaci za formu zbirnog KIR unosa (maloprodaja)
type PodaciDnevniPazar struct {
	model.PodaciStranice
	Datum      string
	Promet     model.DnevniPrometKir
	Ukupno     float64
	VecUpisano bool // true: za ovaj datum već postoji zbirni KIR zapis
}

// DnevniPazarKir prikazuje formu za potvrdu zbirnog KIR unosa za izabrani datum.
// Iznosi se računaju iz maloprodajnih naloga (bez klijenta) tog dana.
func (h *Handler) DnevniPazarKir(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "pdv.dodaj"); !ok {
		return
	}
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	datum := strings.TrimSpace(r.URL.Query().Get("datum"))
	if datum == "" {
		datum = time.Now().Format("2006-01-02")
	}

	promet, err := h.ProdajaRepo.DnevniPrometMaloprodaje(r.Context(), datum)
	if err != nil {
		http.Error(w, "Greška pri računanju dnevnog prometa", http.StatusInternalServerError)
		return
	}

	ukupno := math.Round((promet.OsnovicaOpsta+promet.PdvOpsta+promet.OsnovicaPosebna+promet.PdvPosebna)*100) / 100

	var vecUpisano bool
	if brojDok, e := brojDokumentaDnevniPazar(datum); e == nil {
		vecUpisano, _ = h.PdvKirRepo.PostojiPoBrojuDokumenta(r.Context(), brojDok)
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "pdv-kir"
	ps.NaslovStranice = "Dnevni pazar — KIR unos"
	h.renderujTemplate(w, "pdv_kir_dnevni_pazar", PodaciDnevniPazar{
		PodaciStranice: ps,
		Datum:          datum,
		Promet:         promet,
		Ukupno:         ukupno,
		VecUpisano:     vecUpisano,
	})
}

// brojDokumentaDnevniPazar gradi deterministički broj dokumenta za zbirni KIR unos
// dnevnog pazara ("FISK-YYYYMMDD") — koristi se i za upis i za proveru duplikata.
func brojDokumentaDnevniPazar(datum string) (string, error) {
	d, err := time.Parse("2006-01-02", datum)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("FISK-%s", d.Format("20060102")), nil
}

// SacuvajDnevniPazarKir prima POST i upisuje zbirni KIR zapis za dnevni pazar.
func (h *Handler) SacuvajDnevniPazarKir(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "pdv.dodaj"); !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	datumStr := strings.TrimSpace(r.FormValue("datum"))
	datum, e1 := time.Parse("2006-01-02", datumStr)
	if e1 != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Datum je obavezan.")
		http.Redirect(w, r, "/pdv/kir/dnevni-pazar", http.StatusSeeOther)
		return
	}

	// ako za ovaj dan već postoji zbirni zapis, zameni ga ažuriranim iznosima
	// (npr. naknadni storno ili prodaja posle prethodnog upisa promenili su promet tog dana)
	brojDokumenta := fmt.Sprintf("FISK-%s", datum.Format("20060102"))
	if err := h.PdvKirRepo.ObrisiPoBrojuDokumenta(r.Context(), brojDokumenta); err != nil {
		slog.Error("brisanje starog dnevnog pazara nije uspelo", "datum", datum, "error", err)
	}

	z := model.PdvKir{
		DatumPrometa:    datum,
		DatumKnjizenja:  datum,
		BrojDokumenta:   brojDokumenta,
		KupacNaziv:      "Promet fizičkim licima",
		OsnovicaOpsta:   parsiraIznos(r.FormValue("osnovica_opsta")),
		PdvOpsta:        parsiraIznos(r.FormValue("pdv_opsta")),
		OsnovicaPosebna: parsiraIznos(r.FormValue("osnovica_posebna")),
		PdvPosebna:      parsiraIznos(r.FormValue("pdv_posebna")),
		Napomena:        "Zbirni promet fizičkim licima — fiskalna kasa",
		Izvor:           "rucno",
	}
	z.Ukupno = math.Round((z.OsnovicaOpsta+z.PdvOpsta+z.OsnovicaPosebna+z.PdvPosebna)*100) / 100

	if _, err := h.PdvKirRepo.Kreiraj(r.Context(), &z); err != nil {
		slog.Error("greška pri čuvanju dnevnog pazara u KIR", "datum", datum, "error", err)
		http.Error(w, "Greška pri čuvanju zapisa", http.StatusInternalServerError)
		return
	}
	middleware.SetFlash(w, r, h.DB, "uspeh", fmt.Sprintf("Dnevni pazar za %s upisan u KIR.", datum.Format("02.01.2006.")))
	http.Redirect(w, r, "/pdv/kir", http.StatusSeeOther)
}
