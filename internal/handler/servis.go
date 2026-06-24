package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image/png"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	appdbPkg "ntech/internal/db"
	"ntech/internal/db/sqlite"
	"ntech/internal/middleware"
	"ntech/internal/model"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
	"github.com/go-chi/chi/v5"
	qrcode "github.com/skip2/go-qrcode"
)

// PodaciServisa su podaci za stranicu sa listom servisnih naloga
type PodaciServisa struct {
	model.PodaciStranice
	Nalozi       []model.ServisniNalogSaKlijentom
	Pretraga     string
	FilterStatus string
	SviStatusi   []string
	Sacuvano     bool
	Obrisan      bool
}

// PodaciFormeNaloga su podaci za formu novog/izmenjenog servisnog naloga
type PodaciFormeNaloga struct {
	model.PodaciStranice
	Nalog            model.ServisniNalog
	Klijenti         []model.Klijent
	Tehnicari        []model.Korisnik
	SviStatusi       []string
	Greska           string
	Izmena           bool
	GarancijaDefault string // podrazumevana garancija (prijem + meseci), format 2006-01-02
	BezGarancije     bool   // u bazi nalog nema garanciju (GarancijaDo == NULL)
}

// PodaciDetaljiNaloga su podaci za pregled jednog servisnog naloga
type PodaciDetaljiNaloga struct {
	model.PodaciStranice
	Nalog                   model.ServisniNalog
	KlijentNaziv            string
	TehnicarNaziv           string
	Tehnicari               []model.Korisnik
	GarancijaDefault        string                        // podrazumevana garancija (prijem + meseci), format 2006-01-02
	BezGarancije            bool                          // u bazi nalog nema garanciju (GarancijaDo == NULL)
	ServisniDelovi          []model.ServisniDeoSaArtiklom // ugrađeni/traženi (predlozeno=false)
	ServisniRadovi          []model.ServisniRad           // ugrađeni/traženi (predlozeno=false)
	PredlozeniDelovi        []model.ServisniDeoSaArtiklom // serviser predložio posle dijagnostike
	PredlozeniRadovi        []model.ServisniRad           // serviser predložio posle dijagnostike
	ImaPredlog              bool                          // postoji bar jedna predložena stavka
	ZakljucaniUgradjeni     bool                          // ugrađene stavke zaključane (status nije „Primljeno")
	Artikli                 []model.ArtikalSaKategorijom
	Usluge                  []model.Usluga
	Sacuvano                bool
	UkupnoDelovi            float64
	UkupnoRadovi            float64
	UkupnoSve               float64
	Avans                   float64
	PreostaloSve            float64
	ZakljucanStatus         bool           // onemogući promenu statusa dok ima potraživanih delova
	CenaDijagnostikePredlog string         // podrazumevana cena dijagnostike iz podešavanja (za prefill input-a)
	KorisceneUsluge         map[int64]bool // ID-evi usluga već dodatih na nalog — izostavljaju se iz dropdown-a
	SviStatusi              []string
}

// Servis renderuje listu servisnih naloga sa opcionom pretragom i filterom statusa
func (h *Handler) Servis(w http.ResponseWriter, r *http.Request) {
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	pretraga := r.URL.Query().Get("pretraga")
	filterStatus := r.URL.Query().Get("status")

	nalozi, err := h.ServisRepo.Lista(r.Context(), pretraga, filterStatus)
	if err != nil {
		http.Error(w, "Greška pri učitavanju naloga", http.StatusInternalServerError)
		return
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "servis"
	ps.NaslovStranice = "Servis"
	podaci := PodaciServisa{
		PodaciStranice: ps,
		Nalozi:         nalozi,
		Pretraga:       pretraga,
		FilterStatus:   filterStatus,
		SviStatusi:     model.SviStatusi,
		Sacuvano:       r.URL.Query().Get("sacuvano") == "1",
		Obrisan:        r.URL.Query().Get("obrisan") == "1",
	}

	h.renderujTemplate(w, "servis", podaci)
}

// ArhivaServisa prikazuje preuzete naloge — oni napuštaju kanban tablu i ovde se
// pretražuju, da tabla ostane pregledna i pokazuje samo aktivan rad
func (h *Handler) ArhivaServisa(w http.ResponseWriter, r *http.Request) {
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	pretraga := r.URL.Query().Get("pretraga")
	nalozi, err := h.ServisRepo.Lista(r.Context(), pretraga, model.StatusPreuzeto)
	if err != nil {
		http.Error(w, "Greška pri učitavanju naloga", http.StatusInternalServerError)
		return
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "servis"
	ps.NaslovStranice = "Arhiva servisa"
	podaci := PodaciServisa{
		PodaciStranice: ps,
		Nalozi:         nalozi,
		Pretraga:       pretraga,
		SviStatusi:     model.SviStatusi,
	}

	h.renderujTemplate(w, "servis_arhiva", podaci)
}

// NoviNalog generiše broj naloga i prikazuje praznu formu za unos
func (h *Handler) NoviNalog(w http.ResponseWriter, r *http.Request) {
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	brojNaloga, err := h.ServisRepo.SledeciBroj(r.Context())
	if err != nil {
		http.Error(w, "Greška pri generisanju broja naloga", http.StatusInternalServerError)
		return
	}

	klijenti, err := h.KlijentiRepo.Lista(r.Context(), "")
	if err != nil {
		http.Error(w, "Greška pri učitavanju klijenata", http.StatusInternalServerError)
		return
	}

	tehnicari, err := h.KorisniciRepo.Lista(r.Context())
	if err != nil {
		http.Error(w, "Greška pri učitavanju servisera", http.StatusInternalServerError)
		return
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "servis"
	ps.NaslovStranice = "Novi nalog"
	noviNalog := model.ServisniNalog{
		BrojNaloga:   brojNaloga,
		Status:       model.StatusPrimljeno,
		DatumPrijema: time.Now(),
	}
	noviNalog.GarancijaDo = defaultGarancija(noviNalog.DatumPrijema, podesavanja)
	h.renderujFormuNaloga(w, PodaciFormeNaloga{
		PodaciStranice: ps,
		Nalog:          noviNalog,
		Klijenti:       klijenti,
		Tehnicari:      tehnicari,
		SviStatusi:     model.SviStatusi,
		Izmena:         false,
	})
}

// SacuvajNalog prima POST formu i upisuje novi servisni nalog u bazu
func (h *Handler) SacuvajNalog(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "servis.dodaj"); !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	nalog, greska := parseFormuNaloga(r)
	if greska != "" {
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		klijenti, _ := h.KlijentiRepo.Lista(r.Context(), "")
		tehnicari, _ := h.KorisniciRepo.Lista(r.Context())
		ps := h.popuniPodaciStranice(r, podesavanja)
		ps.Stranica = "servis"
		ps.NaslovStranice = "Novi nalog"
		h.renderujFormuNaloga(w, PodaciFormeNaloga{
			PodaciStranice: ps,
			Nalog:          nalog,
			Klijenti:       klijenti,
			Tehnicari:      tehnicari,
			SviStatusi:     model.SviStatusi,
			Greska:         greska,
			Izmena:         false,
		})
		return
	}

	// forma novog naloga ne sadrži polje garancije — primeni podrazumevanu
	// garanciju iz podešavanja, osim ako je korisnik izričito izabrao „bez garancije"
	if nalog.GarancijaDo == nil && r.FormValue("bez_garancije") != "1" {
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		nalog.GarancijaDo = defaultGarancija(nalog.DatumPrijema, podesavanja)
	}

	// garancija ne sme da bude pre datuma prijema
	if nalog.GarancijaDo != nil && garancijaPrePrijema(*nalog.GarancijaDo, nalog.DatumPrijema) {
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		klijenti, _ := h.KlijentiRepo.Lista(r.Context(), "")
		tehnicari, _ := h.KorisniciRepo.Lista(r.Context())
		ps := h.popuniPodaciStranice(r, podesavanja)
		ps.Stranica = "servis"
		ps.NaslovStranice = "Novi nalog"
		h.renderujFormuNaloga(w, PodaciFormeNaloga{
			PodaciStranice: ps,
			Nalog:          nalog,
			Klijenti:       klijenti,
			Tehnicari:      tehnicari,
			SviStatusi:     model.SviStatusi,
			Greska:         "Datum garancije ne može biti pre datuma prijema.",
			Izmena:         false,
		})
		return
	}

	// ako nije izabran postojeći klijent, eventualno kreiraj novog iz polja forme
	if greska := h.mozdaKreirajKlijenta(r.Context(), r, &nalog); greska != "" {
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		klijenti, _ := h.KlijentiRepo.Lista(r.Context(), "")
		tehnicari, _ := h.KorisniciRepo.Lista(r.Context())
		ps := h.popuniPodaciStranice(r, podesavanja)
		ps.Stranica = "servis"
		ps.NaslovStranice = "Novi nalog"
		h.renderujFormuNaloga(w, PodaciFormeNaloga{
			PodaciStranice: ps,
			Nalog:          nalog,
			Klijenti:       klijenti,
			Tehnicari:      tehnicari,
			SviStatusi:     model.SviStatusi,
			Greska:         greska,
			Izmena:         false,
		})
		return
	}

	id, err := h.ServisRepo.Kreiraj(r.Context(), &nalog)
	if err != nil {
		slog.Error("greška pri čuvanju naloga", "error", err)
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		klijenti, _ := h.KlijentiRepo.Lista(r.Context(), "")
		tehnicari, _ := h.KorisniciRepo.Lista(r.Context())
		ps := h.popuniPodaciStranice(r, podesavanja)
		ps.Stranica = "servis"
		ps.NaslovStranice = "Novi nalog"
		h.renderujFormuNaloga(w, PodaciFormeNaloga{
			PodaciStranice: ps,
			Nalog:          nalog,
			Klijenti:       klijenti,
			Tehnicari:      tehnicari,
			SviStatusi:     model.SviStatusi,
			Greska:         "Došlo je do greške pri čuvanju. Pokušajte ponovo.",
			Izmena:         false,
		})
		return
	}

	http.Redirect(w, r, "/servis/"+strconv.FormatInt(id, 10)+"?sacuvano=1", http.StatusSeeOther)
}

// IzmeniNalog učitava servisni nalog po ID-u i prikazuje popunjenu formu
func (h *Handler) IzmeniNalog(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID naloga", http.StatusBadRequest)
		return
	}

	nalog, err := h.ServisRepo.DohvatiID(r.Context(), id)
	if err != nil {
		http.Error(w, "Nalog nije pronađen", http.StatusNotFound)
		return
	}

	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	klijenti, err := h.KlijentiRepo.Lista(r.Context(), "")
	if err != nil {
		http.Error(w, "Greška pri učitavanju klijenata", http.StatusInternalServerError)
		return
	}

	tehnicari, err := h.KorisniciRepo.Lista(r.Context())
	if err != nil {
		http.Error(w, "Greška pri učitavanju servisera", http.StatusInternalServerError)
		return
	}

	garancijaDefault := ""
	if d := defaultGarancija(nalog.DatumPrijema, podesavanja); d != nil {
		garancijaDefault = d.Format("2006-01-02")
	}
	// stanje „bez garancije" čitamo iz baze pre nego što popunimo default za prikaz
	bezGarancije := nalog.GarancijaDo == nil
	if nalog.GarancijaDo == nil {
		nalog.GarancijaDo = defaultGarancija(nalog.DatumPrijema, podesavanja)
	}
	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "servis"
	ps.NaslovStranice = "Izmeni nalog"
	h.renderujFormuNaloga(w, PodaciFormeNaloga{
		PodaciStranice:   ps,
		Nalog:            *nalog,
		Klijenti:         klijenti,
		Tehnicari:        tehnicari,
		SviStatusi:       model.SviStatusi,
		Izmena:           true,
		GarancijaDefault: garancijaDefault,
		BezGarancije:     bezGarancije,
	})
}

// SacuvajIzmenaNaloga prima POST formu i ažurira postojeći servisni nalog
func (h *Handler) SacuvajIzmenaNaloga(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "servis.izmeni"); !ok {
		return
	}
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID naloga", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	// auto-snimanje (forma se snima u realnom vremenu): umesto redirecta vraćamo 204,
	// a greške kao kratak tekst sa statusom — JS na stranici to prikaže korisniku
	autosave := r.Header.Get("X-Autosave") == "1"

	nalog, greska := parseFormuNaloga(r)
	if greska != "" {
		if autosave {
			http.Error(w, greska, http.StatusBadRequest)
			return
		}
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		klijenti, _ := h.KlijentiRepo.Lista(r.Context(), "")
		tehnicari, _ := h.KorisniciRepo.Lista(r.Context())
		nalog.ID = id
		ps := h.popuniPodaciStranice(r, podesavanja)
		ps.Stranica = "servis"
		ps.NaslovStranice = "Izmeni nalog"
		h.renderujFormuNaloga(w, PodaciFormeNaloga{
			PodaciStranice: ps,
			Nalog:          nalog,
			Klijenti:       klijenti,
			Tehnicari:      tehnicari,
			SviStatusi:     model.SviStatusi,
			Greska:         greska,
			Izmena:         true,
		})
		return
	}

	nalog.ID = id

	// forma izmene više ne šalje status, datume (osim garancije), cene ni klijenta —
	// očuvaj postojeće vrednosti (status se menja u detaljima, cena rada se računa iz
	// radova, datum završetka se postavlja kasnije). Garancija se sada uređuje u formi,
	// pa je čuvamo iz parseFormuNaloga. Inače bi prazna polja pregazila bazu.
	if stari, e := h.ServisRepo.DohvatiID(r.Context(), id); e == nil && stari != nil {
		nalog.Status = stari.Status
		nalog.KlijentID = stari.KlijentID
		nalog.CenaOd = stari.CenaOd
		nalog.CenaDo = stari.CenaDo
		nalog.CenaKonacna = stari.CenaKonacna
		nalog.DatumZavrsetka = stari.DatumZavrsetka
		nalog.PredvidjenDatum = stari.PredvidjenDatum
		nalog.DatumPrijema = stari.DatumPrijema
		nalog.GarancijaDo = stari.GarancijaDo     // garancija se uređuje u detaljima, ne u formi
		nalog.GarancijaDana = stari.GarancijaDana // isto — trajanje garancije se ne dira u formi
	}

	// garancija ne sme da bude pre datuma prijema
	if nalog.GarancijaDo != nil && garancijaPrePrijema(*nalog.GarancijaDo, nalog.DatumPrijema) {
		greska := "Datum garancije ne može biti pre datuma prijema."
		if autosave {
			http.Error(w, greska, http.StatusBadRequest)
			return
		}
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		klijenti, _ := h.KlijentiRepo.Lista(r.Context(), "")
		tehnicari, _ := h.KorisniciRepo.Lista(r.Context())
		ps := h.popuniPodaciStranice(r, podesavanja)
		ps.Stranica = "servis"
		ps.NaslovStranice = "Izmeni nalog"
		h.renderujFormuNaloga(w, PodaciFormeNaloga{
			PodaciStranice: ps,
			Nalog:          nalog,
			Klijenti:       klijenti,
			Tehnicari:      tehnicari,
			SviStatusi:     model.SviStatusi,
			Greska:         greska,
			Izmena:         true,
		})
		return
	}

	// ako nije izabran postojeći klijent, eventualno kreiraj novog iz polja forme
	if greska := h.mozdaKreirajKlijenta(r.Context(), r, &nalog); greska != "" {
		if autosave {
			http.Error(w, greska, http.StatusBadRequest)
			return
		}
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		klijenti, _ := h.KlijentiRepo.Lista(r.Context(), "")
		tehnicari, _ := h.KorisniciRepo.Lista(r.Context())
		ps := h.popuniPodaciStranice(r, podesavanja)
		ps.Stranica = "servis"
		ps.NaslovStranice = "Izmeni nalog"
		h.renderujFormuNaloga(w, PodaciFormeNaloga{
			PodaciStranice: ps,
			Nalog:          nalog,
			Klijenti:       klijenti,
			Tehnicari:      tehnicari,
			SviStatusi:     model.SviStatusi,
			Greska:         greska,
			Izmena:         true,
		})
		return
	}

	if err := h.ServisRepo.Izmeni(r.Context(), &nalog); err != nil {
		slog.Error("greška pri čuvanju izmene naloga", "error", err)
		if autosave {
			http.Error(w, "Došlo je do greške pri čuvanju.", http.StatusInternalServerError)
			return
		}
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		klijenti, _ := h.KlijentiRepo.Lista(r.Context(), "")
		tehnicari, _ := h.KorisniciRepo.Lista(r.Context())
		ps := h.popuniPodaciStranice(r, podesavanja)
		ps.Stranica = "servis"
		ps.NaslovStranice = "Izmeni nalog"
		h.renderujFormuNaloga(w, PodaciFormeNaloga{
			PodaciStranice: ps,
			Nalog:          nalog,
			Klijenti:       klijenti,
			Tehnicari:      tehnicari,
			SviStatusi:     model.SviStatusi,
			Greska:         "Došlo je do greške pri čuvanju. Pokušajte ponovo.",
			Izmena:         true,
		})
		return
	}

	if autosave {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, "/servis/"+strconv.FormatInt(id, 10)+"?sacuvano=1", http.StatusSeeOther)
}

// ObrisiNalog prima POST zahtev i briše servisni nalog po ID-u
func (h *Handler) ObrisiNalog(w http.ResponseWriter, r *http.Request) {
	k, ok := h.zahtevajDozvolu(w, r, "servis.obrisi")
	if !ok {
		return
	}
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID naloga", http.StatusBadRequest)
		return
	}

	if err := h.ServisRepo.Obrisi(r.Context(), id, &k.ID); err != nil {
		slog.Error("greška pri brisanju naloga", "id", id, "error", err)
		http.Error(w, "Greška pri brisanju naloga", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/servis?obrisan=1", http.StatusSeeOther)
}

// DetaljiNaloga prikazuje sve podatke jednog servisnog naloga sa ugrađenim delovima
func (h *Handler) DetaljiNaloga(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID naloga", http.StatusBadRequest)
		return
	}

	nalog, err := h.ServisRepo.DohvatiID(r.Context(), id)
	if err != nil {
		http.Error(w, "Nalog nije pronađen", http.StatusNotFound)
		return
	}

	// self-heal: ako u međuvremenu ima stanja na magacinu za potraživane artikle,
	// povuci ih (delimično ili u celosti) i otključaj nalog ako je sve pokriveno.
	// Hvata sve načine na koje stanje poraste, ne samo nabavku/izmenu/popis.
	if potrazivani, e := h.ServisniPotrazivaniDeloviRepo.DohvatiZaNalog(r.Context(), id); e == nil && len(potrazivani) > 0 {
		vidjeni := map[int64]bool{}
		for _, p := range potrazivani {
			if vidjeni[p.ArtikalID] {
				continue
			}
			vidjeni[p.ArtikalID] = true
			otkljucani, err := h.ServisniPotrazivaniDeloviRepo.ProveriIPocistiZaArtikal(r.Context(), p.ArtikalID)
			if err != nil {
				slog.Error("self-heal potraživanih delova nije uspeo", "artikal_id", p.ArtikalID, "error", err)
				continue
			}
			for _, nalogID := range otkljucani {
				// reset samo ako je nalog čekao delove — ne sme da menja U dijagnostici, U popravci itd.
				if tekuci, e := h.ServisRepo.DohvatiID(r.Context(), nalogID); e == nil && tekuci != nil && tekuci.Status == model.StatusCekaDelove {
					if err := h.ServisRepo.AzurirajStatus(r.Context(), nalogID, model.StatusPrimljeno); err != nil {
						slog.Error("self-heal reset statusa naloga nije uspeo", "nalog_id", nalogID, "error", err)
					}
				}
			}
		}
		// status naloga se možda promenio — ponovo ga učitaj za prikaz
		if osvezen, e := h.ServisRepo.DohvatiID(r.Context(), id); e == nil && osvezen != nil {
			nalog = osvezen
		}
	}

	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	// predviđen datum popravke: ako postoji ručni override (iz baze) koristi se,
	// inače izvedeni default (datum prijema + rok iz podešavanja)
	if nalog.PredvidjenDatum == nil {
		nalog.PredvidjenDatum = defaultPredvidjenDatum(nalog.DatumPrijema, podesavanja)
	}

	// podrazumevano trajanje garancije u danima (dugme „Podrazumevano") i stanje „bez garancije" iz baze
	garancijaDefault := strconv.Itoa(defaultGarancijaDana(podesavanja))
	bezGarancije := nalog.GarancijaDana == nil || *nalog.GarancijaDana <= 0

	klijentNaziv := ""
	if nalog.KlijentID != nil {
		klijent, err := h.KlijentiRepo.DohvatiID(r.Context(), *nalog.KlijentID)
		if err == nil {
			klijentNaziv = klijent.PunoIme()
		}
	}

	tehnicarNaziv := ""
	if nalog.TehnicarID != nil {
		tehnicar, err := h.KorisniciRepo.DohvatiPoID(r.Context(), *nalog.TehnicarID)
		if err == nil {
			tehnicarNaziv = tehnicar.KorisnickoIme
		}
	}

	// lista servisera za izbor u popup-u „Promeni servisera"
	tehnicari, err := h.KorisniciRepo.Lista(r.Context())
	if err != nil {
		slog.Error("greška pri učitavanju servisera", "error", err)
	}

	delovi, err := h.deloviSaPotrazivanima(r.Context(), id)
	if err != nil {
		slog.Error("greška pri učitavanju delova", "error", err)
	}

	radovi, err := h.ServisniRadoviRepo.DohvatiZaNalog(r.Context(), id)
	if err != nil {
		slog.Error("greška pri učitavanju radova", "error", err)
	}

	// razdvajamo na ugrađene/tražene (predlozeno=false) i predložene (predlozeno=true)
	var ugradjeniDelovi, predlozeniDelovi []model.ServisniDeoSaArtiklom
	for _, d := range delovi {
		if d.Predlozeno {
			predlozeniDelovi = append(predlozeniDelovi, d)
		} else {
			ugradjeniDelovi = append(ugradjeniDelovi, d)
		}
	}
	var ugradjeniRadovi, predlozeniRadovi []model.ServisniRad
	for _, rad := range radovi {
		if rad.Predlozeno {
			predlozeniRadovi = append(predlozeniRadovi, rad)
		} else {
			ugradjeniRadovi = append(ugradjeniRadovi, rad)
		}
	}

	appdb := appdbPkg.ArtikalFilter{}
	artikli, err := h.Artikli.Lista(r.Context(), appdb)
	if err != nil {
		slog.Error("greška pri učitavanju artikala", "error", err)
	}

	usluge, err := h.UslugeRepo.Lista(r.Context(), appdbPkg.UslugaFilter{})
	if err != nil {
		slog.Error("greška pri učitavanju usluga", "error", err)
	}

	var ukupnoDelovi, ukupnoRadovi float64
	for _, d := range delovi {
		ukupnoDelovi += d.Ukupno()
	}
	// skup već dodatih usluga — izostavljaju se iz dropdown-a da se ista usluga ne doda dvaput
	korisceneUsluge := make(map[int64]bool, len(radovi))
	for _, rad := range radovi {
		ukupnoRadovi += rad.Ukupno()
		if rad.UslugaID != 0 {
			korisceneUsluge[rad.UslugaID] = true
		}
	}
	// ukupno za klijenta zavisi od ishoda dijagnostike:
	// – klijent odbio popravku → naplaćuje se samo cena dijagnostike
	// – inače (popravka u toku/gotova) → radovi + delovi, dijagnostika se ne gleda
	var ukupnoSve float64
	if nalog.PopravkaOdbijena {
		ukupnoSve = nalog.CenaDijagnostike
	} else {
		ukupnoSve = ukupnoRadovi + ukupnoDelovi
	}
	avans := 0.0
	if nalog.Avans != nil {
		avans = *nalog.Avans
	}
	preostaloSve := ukupnoSve - avans
	if preostaloSve < 0 {
		preostaloSve = 0
	}

	zakljucanStatus := false
	for _, d := range delovi {
		if d.Potrazivano > 0 {
			zakljucanStatus = true
			break
		}
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "servis"
	ps.NaslovStranice = "Detalji naloga"
	podaci := PodaciDetaljiNaloga{
		PodaciStranice:          ps,
		Nalog:                   *nalog,
		KlijentNaziv:            klijentNaziv,
		TehnicarNaziv:           tehnicarNaziv,
		Tehnicari:               tehnicari,
		GarancijaDefault:        garancijaDefault,
		BezGarancije:            bezGarancije,
		ServisniDelovi:          ugradjeniDelovi,
		ServisniRadovi:          ugradjeniRadovi,
		PredlozeniDelovi:        predlozeniDelovi,
		PredlozeniRadovi:        predlozeniRadovi,
		ImaPredlog:              len(predlozeniDelovi) > 0 || len(predlozeniRadovi) > 0,
		ZakljucaniUgradjeni:     jePredlogStatus(nalog.Status),
		Artikli:                 artikli,
		Usluge:                  usluge,
		Sacuvano:                r.URL.Query().Get("sacuvano") == "1",
		UkupnoDelovi:            ukupnoDelovi,
		UkupnoRadovi:            ukupnoRadovi,
		UkupnoSve:               ukupnoSve,
		Avans:                   avans,
		PreostaloSve:            preostaloSve,
		ZakljucanStatus:         zakljucanStatus,
		CenaDijagnostikePredlog: podesavanja["servis_cena_dijagnostike"],
		KorisceneUsluge:         korisceneUsluge,
		SviStatusi:              model.SviStatusi,
	}

	h.renderujTemplate(w, "servis_detalji", podaci)
}

// jePredlogStatus vraća true ako se stavka dodata u datom statusu računa kao
// predlog servisera (sve posle prijema); dok je nalog „Primljeno" stavke su
// deo zahteva klijenta (ugrađene/tražene).
func jePredlogStatus(status string) bool {
	return status != "" && status != model.StatusPrimljeno
}

// DodajDeloNalogu prima POST formu i dodaje artikal kao deo servisnog naloga
func (h *Handler) DodajDeloNalogu(w http.ResponseWriter, r *http.Request) {
	k, ok := h.zahtevajDozvolu(w, r, "servis.izmeni")
	if !ok {
		return
	}

	nalogID, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID naloga", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	artikalID, err := strconv.ParseInt(r.FormValue("artikal_id"), 10, 64)
	if err != nil || artikalID <= 0 {
		middleware.SetFlash(w, r, h.DB, "greska", "Neispravan artikal.")
		http.Redirect(w, r, "/servis/"+strconv.FormatInt(nalogID, 10), http.StatusSeeOther)
		return
	}

	kolicina, err := strconv.Atoi(r.FormValue("kolicina"))
	if err != nil || kolicina <= 0 {
		middleware.SetFlash(w, r, h.DB, "greska", "Količina mora biti pozitivan broj.")
		http.Redirect(w, r, "/servis/"+strconv.FormatInt(nalogID, 10), http.StatusSeeOther)
		return
	}

	cena, err := strconv.ParseFloat(r.FormValue("cena_komada"), 64)
	if err != nil || cena < 0 {
		middleware.SetFlash(w, r, h.DB, "greska", "Cena mora biti pozitivan broj.")
		http.Redirect(w, r, "/servis/"+strconv.FormatInt(nalogID, 10), http.StatusSeeOther)
		return
	}

	// ime artikla za poruke korisniku
	imeArtikla := ""
	if art, e := h.Artikli.DohvatiID(r.Context(), artikalID); e == nil && art != nil {
		imeArtikla = art.Naziv
	}

	// predlog ako status nije „Primljeno" (forma više ne određuje — server odlučuje)
	nalog, _ := h.ServisRepo.DohvatiID(r.Context(), nalogID)
	predlozeno := nalog != nil && nalog.Status != model.StatusPrimljeno
	slog.Info("DODAJ_DEO_IN", "status", nalog.Status, "predlozeno", predlozeno, "kol", kolicina)
	// atomično: ugradi ono što imamo (skida sa lagera, ne ide u minus), višak u potraživane
	ugradjeno, nedostaje, err := h.ServisniDeloviRepo.UgradiIliPotrazuj(r.Context(), nalogID, artikalID, kolicina, cena, &k.ID, predlozeno)
	slog.Info("DODAJ_DEO_OUT", "ugradjeno", ugradjeno, "nedostaje", nedostaje, "err", err)
	if err != nil {
		slog.Error("greška pri dodavanju dela", "error", err)
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri dodavanju dela.")
		http.Redirect(w, r, "/servis/"+strconv.FormatInt(nalogID, 10), http.StatusSeeOther)
		return
	}

	// ako nešto nedostaje kod ugrađenog dela, prebaci nalog u „Čeka delove".
	// Za predložene delove ne menjamo status — to su samo predlozi, ne blokiraju rad.
	if nedostaje > 0 && !predlozeno {
		if e := h.ServisRepo.AzurirajStatus(r.Context(), nalogID, model.StatusCekaDelove); e != nil {
			slog.Error("greška pri prebacivanju naloga u Čeka delove", "error", e)
		}
		if ugradjeno > 0 {
			middleware.SetFlash(w, r, h.DB, "greska",
				"Ugrađeno "+strconv.Itoa(ugradjeno)+" kom. artikla „"+imeArtikla+
					"\", nedostaje još "+strconv.Itoa(nedostaje)+" — nalog je prebačen u „Čeka delove\".")
		} else {
			middleware.SetFlash(w, r, h.DB, "greska",
				"Nema „"+imeArtikla+"\" na stanju — potrebno "+strconv.Itoa(nedostaje)+
					" kom. Nalog je prebačen u „Čeka delove\".")
		}
	}

	if nedostaje > 0 && predlozeno {
		middleware.SetFlash(w, r, h.DB, "uspeh",
			"Predlog sačuvan — "+strconv.Itoa(kolicina)+" kom. „"+imeArtikla+
				"\" čeka odobrenje klijenta.")
		// novi predlog poništava prethodnu odluku klijenta
		if err := h.ServisRepo.ObrisiOdlukuKlijenta(r.Context(), nalogID); err != nil {
			slog.Error("greška pri brisanju odluke klijenta", "error", err)
		}
	}

	http.Redirect(w, r, "/servis/"+strconv.FormatInt(nalogID, 10)+"?sacuvano=1", http.StatusSeeOther)
}

// deloviSaPotrazivanima vraća ugrađene delove spojene sa potraživanima (ista tabela).
// Potraživani delovi bez ugrađenog (stanje bilo 0) se dodaju sa Kolicina=0.
func (h *Handler) deloviSaPotrazivanima(ctx context.Context, nalogID int64) ([]model.ServisniDeoSaArtiklom, error) {
	delovi, err := h.ServisniDeloviRepo.DohvatiZaNalog(ctx, nalogID)
	if err != nil {
		return nil, err
	}
	potrazivani, err := h.ServisniPotrazivaniDeloviRepo.DohvatiZaNalog(ctx, nalogID)
	if err != nil {
		return delovi, nil // ne prekidamo ako potraživani nisu dostupni
	}
	// ključ uključuje predlozeno: isti artikal može biti i ugrađen i predložen,
	// pa se potraživane količine ne smeju mešati između te dve grupe
	type kljucDela struct {
		artikalID  int64
		predlozeno bool
	}
	type potrVrednost struct {
		kolicina int
		cena     float64
	}
	potrMap := make(map[kljucDela]potrVrednost)
	for _, p := range potrazivani {
		k := kljucDela{p.ArtikalID, p.Predlozeno}
		v := potrMap[k]
		v.kolicina += p.Kolicina
		if v.cena == 0 && p.CenaKomada > 0 {
			v.cena = p.CenaKomada
		}
		potrMap[k] = v
	}
	for i := range delovi {
		k := kljucDela{delovi[i].ArtikalID, delovi[i].Predlozeno}
		if v, ok := potrMap[k]; ok {
			delovi[i].Potrazivano = v.kolicina
			delete(potrMap, k)
		}
	}
	for k, v := range potrMap {
		naziv := ""
		sifra := ""
		if art, e := h.Artikli.DohvatiID(ctx, k.artikalID); e == nil && art != nil {
			naziv = art.Naziv
			sifra = art.Sifra
		}
		delovi = append(delovi, model.ServisniDeoSaArtiklom{
			ArtikalNaziv: naziv,
			ArtikalSifra: sifra,
			ServisniDeo: model.ServisniDeo{
				ArtikalID:  k.artikalID,
				Kolicina:   v.kolicina,
				CenaKomada: v.cena,
				Predlozeno: k.predlozeno,
			},
		})
	}
	return delovi, nil
}

// ObrisiDeloNaloga prima POST zahtev i uklanja deo iz servisnog naloga
func (h *Handler) ObrisiDeloNaloga(w http.ResponseWriter, r *http.Request) {
	k, ok := h.zahtevajDozvolu(w, r, "servis.izmeni")
	if !ok {
		return
	}

	nalogID, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID naloga", http.StatusBadRequest)
		return
	}

	deoID, err := parseID(chi.URLParam(r, "deo_id"))
	if err != nil {
		http.Error(w, "Neispravan ID dela", http.StatusBadRequest)
		return
	}

	// obriši i potraživane delove za isti artikal (inače ostaju blokirani u tabeli)
	if artikalID, err := h.ServisniDeloviRepo.DohvatiArtikalID(r.Context(), deoID); err == nil {
		if err := h.ServisniPotrazivaniDeloviRepo.ObrisiZaArtikal(r.Context(), nalogID, artikalID); err != nil {
			slog.Error("greška pri brisanju potraživanih delova", "error", err)
		}
	}

	if err := h.ServisniDeloviRepo.Obrisi(r.Context(), deoID, &k.ID); err != nil {
		slog.Error("greška pri brisanju dela", "error", err)
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri uklanjanju dela.")
	}
	http.Redirect(w, r, "/servis/"+strconv.FormatInt(nalogID, 10), http.StatusSeeOther)
}

// ObrisiPredlozeniDeoNaloga briše predlozene redove (predlozeno=1) za dati artikal na nalogu
func (h *Handler) ObrisiPredlozeniDeoNaloga(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "servis.izmeni"); !ok {
		return
	}
	nalogID, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID naloga", http.StatusBadRequest)
		return
	}
	artikalID, err := parseID(chi.URLParam(r, "artikal_id"))
	if err != nil {
		http.Error(w, "Neispravan ID artikla", http.StatusBadRequest)
		return
	}
	if err := h.ServisniPotrazivaniDeloviRepo.ObrisiPredlozeneZaArtikal(r.Context(), nalogID, artikalID); err != nil {
		slog.Error("greška pri brisanju predloženog dela", "error", err)
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri uklanjanju predloženog dela.")
	}
	http.Redirect(w, r, "/servis/"+strconv.FormatInt(nalogID, 10), http.StatusSeeOther)
}

// DodajRadNalogu dodaje stavku rada (uslugu) na nalog; naziv i cena se snapshot-uju
func (h *Handler) DodajRadNalogu(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "servis.izmeni"); !ok {
		return
	}

	nalogID, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID naloga", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	uslugaID, err := strconv.ParseInt(r.FormValue("usluga_id"), 10, 64)
	if err != nil || uslugaID <= 0 {
		middleware.SetFlash(w, r, h.DB, "greska", "Izaberite uslugu.")
		http.Redirect(w, r, "/servis/"+strconv.FormatInt(nalogID, 10), http.StatusSeeOther)
		return
	}

	usluga, err := h.UslugeRepo.DohvatiID(r.Context(), uslugaID)
	if err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Izabrana usluga nije pronađena.")
		http.Redirect(w, r, "/servis/"+strconv.FormatInt(nalogID, 10), http.StatusSeeOther)
		return
	}

	kolicina, err := strconv.ParseFloat(r.FormValue("kolicina"), 64)
	if err != nil || kolicina <= 0 {
		middleware.SetFlash(w, r, h.DB, "greska", "Količina mora biti pozitivan broj.")
		http.Redirect(w, r, "/servis/"+strconv.FormatInt(nalogID, 10), http.StatusSeeOther)
		return
	}

	cena, err := strconv.ParseFloat(r.FormValue("cena_komada"), 64)
	if err != nil || cena < 0 {
		middleware.SetFlash(w, r, h.DB, "greska", "Cena mora biti pozitivan broj.")
		http.Redirect(w, r, "/servis/"+strconv.FormatInt(nalogID, 10), http.StatusSeeOther)
		return
	}

	// predlog ako status nije „Primljeno" (server odlučuje, ne forma)
	nalog, _ := h.ServisRepo.DohvatiID(r.Context(), nalogID)
	predlozeno := nalog != nil && nalog.Status != model.StatusPrimljeno

	if _, err := h.ServisniRadoviRepo.Dodaj(r.Context(), nalogID, uslugaID, usluga.Naziv, kolicina, cena, predlozeno); err != nil {
		slog.Error("greška pri dodavanju rada", "error", err)
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri dodavanju rada.")
		http.Redirect(w, r, "/servis/"+strconv.FormatInt(nalogID, 10), http.StatusSeeOther)
		return
	}

	// novi predlog poništava prethodnu odluku klijenta
	if predlozeno {
		if err := h.ServisRepo.ObrisiOdlukuKlijenta(r.Context(), nalogID); err != nil {
			slog.Error("greška pri brisanju odluke klijenta", "error", err)
		}
	}

	http.Redirect(w, r, "/servis/"+strconv.FormatInt(nalogID, 10)+"?sacuvano=1", http.StatusSeeOther)
}

// ObrisiRadNaloga uklanja stavku rada sa naloga
func (h *Handler) ObrisiRadNaloga(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "servis.izmeni"); !ok {
		return
	}

	nalogID, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID naloga", http.StatusBadRequest)
		return
	}

	radID, err := parseID(chi.URLParam(r, "rad_id"))
	if err != nil {
		http.Error(w, "Neispravan ID rada", http.StatusBadRequest)
		return
	}

	if err := h.ServisniRadoviRepo.Obrisi(r.Context(), radID); err != nil {
		slog.Error("greška pri brisanju rada", "error", err)
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri uklanjanju rada.")
	}
	http.Redirect(w, r, "/servis/"+strconv.FormatInt(nalogID, 10), http.StatusSeeOther)
}

// parseFormuNaloga čita i validira polja iz HTTP forme
func parseFormuNaloga(r *http.Request) (model.ServisniNalog, string) {
	uredjaj := strings.TrimSpace(r.FormValue("uredjaj"))
	if uredjaj == "" {
		return model.ServisniNalog{}, "Naziv uređaja je obavezan."
	}

	opisKvara := strings.TrimSpace(r.FormValue("opis_kvara"))
	if opisKvara == "" {
		return model.ServisniNalog{}, "Opis kvara je obavezan."
	}

	nalog := model.ServisniNalog{
		BrojNaloga:         strings.TrimSpace(r.FormValue("broj_naloga")),
		Uredjaj:            uredjaj,
		SerijskiBroj:       strings.TrimSpace(r.FormValue("serijski_broj")),
		OpisKvara:          opisKvara,
		TrazeneNadogradnje: strings.TrimSpace(r.FormValue("trazene_nadogradnje")),
		Status:             r.FormValue("status"),
		Napomena:           strings.TrimSpace(r.FormValue("napomena")),
		Ostecenja:          strings.TrimSpace(r.FormValue("ostecenja")),
		PinUredjaja:        strings.TrimSpace(r.FormValue("pin_uredjaja")),
		Pribor:             strings.TrimSpace(r.FormValue("pribor")),
		DatumPrijema:       time.Now(),
	}

	// datum prijema — korisnik može da unese drugi datum (npr. retroaktivno)
	if dp := strings.TrimSpace(r.FormValue("datum_prijema")); dp != "" {
		if t, err := time.Parse("2006-01-02", dp); err == nil {
			nalog.DatumPrijema = t
		}
	}

	if nalog.Status == "" {
		nalog.Status = model.StatusPrimljeno
	}

	// opcioni klijent
	if kidStr := r.FormValue("klijent_id"); kidStr != "" {
		if kid, err := strconv.ParseInt(kidStr, 10, 64); err == nil {
			nalog.KlijentID = &kid
		}
	}

	// opcioni serviser
	if tidStr := r.FormValue("tehnicar_id"); tidStr != "" {
		if tid, err := strconv.ParseInt(tidStr, 10, 64); err == nil {
			nalog.TehnicarID = &tid
		}
	}

	// opcione cene — prazno polje ostaje nil
	nalog.CenaOd = parseOpcionuCenu(r.FormValue("cena_od"))
	nalog.CenaDo = parseOpcionuCenu(r.FormValue("cena_do"))
	nalog.CenaKonacna = parseOpcionuCenu(r.FormValue("cena_konacna"))
	nalog.Avans = parseOpcionuCenu(r.FormValue("avans"))

	// opcioni datum završetka
	if dz := strings.TrimSpace(r.FormValue("datum_zavrsetka")); dz != "" {
		if t, err := time.Parse("2006-01-02", dz); err == nil {
			nalog.DatumZavrsetka = &t
		}
	}

	// opcioni datum garancije — preskačemo ako je korisnik označio "bez garancije"
	if r.FormValue("bez_garancije") != "1" {
		if gd := strings.TrimSpace(r.FormValue("garancija_do")); gd != "" {
			if t, err := time.Parse("2006-01-02", gd); err == nil {
				nalog.GarancijaDo = &t
			}
		}
	}

	return nalog, ""
}

// mozdaKreirajKlijenta kreira novog klijenta iz polja forme naloga kada nije
// izabran postojeći (klijent_id prazno), a uneto je ime (fizičko) ili naziv firme
// (pravno). Klijent je opcioni — ako nema dovoljno podataka, nalog ostaje bez njega.
// Vraća srpsku poruku o grešci za prikaz, ili prazan string ako je sve u redu.
func (h *Handler) mozdaKreirajKlijenta(ctx context.Context, r *http.Request, nalog *model.ServisniNalog) string {
	if nalog.KlijentID != nil {
		return "" // izabran postojeći klijent — ništa ne kreiramo
	}

	tip := r.FormValue("tip")
	if tip != "fizicko" && tip != "pravno" {
		tip = "fizicko"
	}
	ime := strings.TrimSpace(r.FormValue("ime"))
	nazivFirme := strings.TrimSpace(r.FormValue("naziv_firme"))

	// bez minimalnih podataka nalog ostaje bez klijenta
	if tip == "fizicko" && ime == "" {
		return ""
	}
	if tip == "pravno" && nazivFirme == "" {
		return ""
	}

	email := strings.TrimSpace(r.FormValue("email"))
	if email != "" && !strings.Contains(email, "@") {
		return "Adresa e-pošte nije ispravna."
	}

	// tip identifikacionog broja se prepoznaje iz dužine unosa:
	// 13 cifara → JMBG, 9 cifara → broj lične karte
	jmbg := strings.TrimSpace(r.FormValue("jmbg"))
	tipIdent, greska := odrediTipIdentifikacije(jmbg)
	if greska != "" {
		return greska
	}
	if tipIdent == "" {
		tipIdent = "jmbg" // prazan unos — podrazumevani tip radi konzistentnosti baze
	}

	prezime := strings.TrimSpace(r.FormValue("prezime"))
	telefon := strings.TrimSpace(r.FormValue("telefon"))
	mestoLok := strings.TrimSpace(r.FormValue("mesto"))

	// prvo proveri da li klijent već postoji u bazi
	postojeci, err := h.KlijentiRepo.Pronadji(ctx, tip, ime, prezime, nazivFirme, jmbg, telefon, email, mestoLok)
	if err != nil {
		slog.Error("greška pri pretrazi klijenta", "error", err)
		return "Došlo je do greške pri proveri klijenta. Pokušajte ponovo."
	}
	if postojeci != nil {
		nalog.KlijentID = &postojeci.ID
		return ""
	}

	// klijent ne postoji — kreiraj novog
	klijent := model.Klijent{
		Tip:               tip,
		Ime:               ime,
		Prezime:           prezime,
		JMBG:              jmbg,
		TipIdentifikacije: tipIdent,
		NazivFirme:        nazivFirme,
		PIB:               strings.TrimSpace(r.FormValue("pib")),
		Telefon:           strings.TrimSpace(r.FormValue("telefon")),
		Email:             email,
		Mesto:             strings.TrimSpace(r.FormValue("mesto")),
	}

	id, err := h.KlijentiRepo.Kreiraj(ctx, &klijent)
	if err != nil {
		slog.Error("greška pri kreiranju klijenta iz naloga", "error", err)
		return "Došlo je do greške pri čuvanju klijenta. Pokušajte ponovo."
	}
	nalog.KlijentID = &id
	return ""
}

// defaultGarancija vraća datum garancije na osnovu datuma prijema i podešavanja;
// vraća nil ako je rok 0 ili podešavanje nedostaje
func defaultGarancija(datumPrijema time.Time, podesavanja map[string]string) *time.Time {
	meseci, err := strconv.Atoi(vrednostIliDefault(podesavanja, "servis_garancija_meseci", "2"))
	if err != nil || meseci <= 0 {
		return nil
	}
	t := datumPrijema.AddDate(0, meseci, 0)
	return &t
}

// defaultPredvidjenDatum računa predviđen rok popravke kao datum prijema + broj
// dana iz podešavanja (predvidjen_rok_dana, podrazumevano 15)
func defaultPredvidjenDatum(datumPrijema time.Time, podesavanja map[string]string) *time.Time {
	dana, err := strconv.Atoi(vrednostIliDefault(podesavanja, "predvidjen_rok_dana", "15"))
	if err != nil || dana <= 0 {
		return nil
	}
	t := datumPrijema.AddDate(0, 0, dana)
	return &t
}

// srpskiPlural vraća odgovarajući oblik reči za dati broj (1 / 2–4 / 5+).
func srpskiPlural(n int, jedan, par, mnogo string) string {
	if n < 0 {
		n = -n
	}
	d, dd := n%10, n%100
	switch {
	case d == 1 && dd != 11:
		return jedan
	case d >= 2 && d <= 4 && (dd < 12 || dd > 14):
		return par
	default:
		return mnogo
	}
}

// garancijaUTekst formatira trajanje garancije (u danima) na srpskom:
// 20 → „20 dana", 45 → „1 mesec i 15 dana", 60 → „2 meseca".
func garancijaUTekst(dana int) string {
	if dana <= 0 {
		return "bez garancije"
	}
	m := dana / 30
	d := dana % 30
	mesecRec := srpskiPlural(m, "mesec", "meseca", "meseci")
	danRec := srpskiPlural(d, "dan", "dana", "dana")
	switch {
	case m > 0 && d > 0:
		return fmt.Sprintf("%d %s i %d %s", m, mesecRec, d, danRec)
	case m > 0:
		return fmt.Sprintf("%d %s", m, mesecRec)
	default:
		return fmt.Sprintf("%d %s", d, danRec)
	}
}

// garancijaTekst je šablonski helper: trajanje garancije u danima → tekst, ili „—" za prazno.
func garancijaTekst(dana *int) string {
	if dana == nil || *dana <= 0 {
		return "—"
	}
	return garancijaUTekst(*dana)
}

// defaultGarancijaDana vraća podrazumevano trajanje garancije u danima iz podešavanja
// (servis_garancija_dana, podrazumevano 60).
func defaultGarancijaDana(podesavanja map[string]string) int {
	dana, err := strconv.Atoi(vrednostIliDefault(podesavanja, "servis_garancija_dana", "60"))
	if err != nil || dana < 0 {
		return 60
	}
	return dana
}

// garancijaPrePrijema vraća true ako je datum garancije raniji od datuma prijema (po danu).
// Garancija ne sme da bude pre prijema uređaja.
func garancijaPrePrijema(garancija, prijem time.Time) bool {
	g := time.Date(garancija.Year(), garancija.Month(), garancija.Day(), 0, 0, 0, 0, time.UTC)
	p := time.Date(prijem.Year(), prijem.Month(), prijem.Day(), 0, 0, 0, 0, time.UTC)
	return g.Before(p)
}

// parseOpcionuCenu pretvara string u *float64 — prazno polje ili neispravna vrednost vraća nil
func parseOpcionuCenu(s string) *float64 {
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

// renderujFormuNaloga renderuje HTML šablon forme za unos ili izmenu servisnog naloga
func (h *Handler) renderujFormuNaloga(w http.ResponseWriter, podaci PodaciFormeNaloga) {
	h.renderujTemplate(w, "servis_forma", podaci)
}

// PodaciRadnogNaloga su podaci za interni radni list servisera (bez cena)
// logoZaDokument vraća putanju loga za štampu na dokumentima — samo ako je u
// Podešavanja → Opšte uključeno „Prikaži logo" (topbar_logo_slika). Inače prazno.
func logoZaDokument(podesavanja map[string]string) string {
	if podesavanja["topbar_logo_slika"] == "1" {
		return podesavanja["logo_putanja"]
	}
	return ""
}

type PodaciRadnogNaloga struct {
	Nalog          model.ServisniNalog
	Radovi         []model.ServisniRad
	ServisniDelovi []model.ServisniDeoSaArtiklom
	Klijent        *model.Klijent
	KlijentNaziv   string
	TehnicarNaziv  string
	NazivFirme     string
	LogoPutanja    string
	Barkod         string // base64 PNG Code128 barkoda broja naloga (za skener)
}

// StampaRadnogNaloga renderuje interni radni list koji serviser preuzima uz uređaj
func (h *Handler) StampaRadnogNaloga(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID naloga", http.StatusBadRequest)
		return
	}

	nalog, err := h.ServisRepo.DohvatiID(r.Context(), id)
	if err != nil {
		http.Error(w, "Nalog nije pronađen", http.StatusNotFound)
		return
	}

	radovi, err := h.ServisniRadoviRepo.DohvatiZaNalog(r.Context(), id)
	if err != nil {
		http.Error(w, "Greška pri učitavanju radova", http.StatusInternalServerError)
		return
	}

	delovi, err := h.deloviSaPotrazivanima(r.Context(), id)
	if err != nil {
		http.Error(w, "Greška pri učitavanju delova", http.StatusInternalServerError)
		return
	}

	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	var klijent *model.Klijent
	klijentNaziv := ""
	if nalog.KlijentID != nil {
		k, err := h.KlijentiRepo.DohvatiID(r.Context(), *nalog.KlijentID)
		if err == nil {
			klijent = k
			if k.NazivFirme != "" {
				klijentNaziv = k.NazivFirme
			} else {
				klijentNaziv = strings.TrimSpace(k.Ime + " " + k.Prezime)
			}
		}
	}

	tehnicarNaziv := ""
	if nalog.TehnicarID != nil {
		tehnicar, err := h.KorisniciRepo.DohvatiPoID(r.Context(), *nalog.TehnicarID)
		if err == nil {
			tehnicarNaziv = tehnicar.KorisnickoIme
		}
	}

	h.renderujStandalone(w, "servis_radni_nalog", PodaciRadnogNaloga{
		Nalog:          *nalog,
		Radovi:         radovi,
		ServisniDelovi: delovi,
		Klijent:        klijent,
		KlijentNaziv:   klijentNaziv,
		TehnicarNaziv:  tehnicarNaziv,
		NazivFirme:     podesavanja["naziv_firme"],
		LogoPutanja:    logoZaDokument(podesavanja),
		Barkod:         barkodNaloga(nalog.BrojNaloga),
	})
}

// PodaciOtpremnice su podaci za otpremnicu pri preuzimanju uređaja
type PodaciOtpremnice struct {
	Nalog          model.ServisniNalog
	ServisniDelovi []model.ServisniDeoSaArtiklom
	UkupnoDelovi   float64
	PreostaloSve   float64
	ImaAvans       bool
	QRKod          string
	Klijent        *model.Klijent
	KlijentNaziv   string
	TehnicarNaziv  string
	NazivFirme     string
	LogoPutanja    string
	Podnazlov      string
	Adresa         string
	Telefon        string
	PIB            string
	MaticniBroj    string
	Barkod         string // base64 PNG Code128 barkoda broja naloga
}

// StampaOtpremnice renderuje otpremnicu pri preuzimanju uređaja od strane klijenta
func (h *Handler) StampaOtpremnice(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID naloga", http.StatusBadRequest)
		return
	}

	nalog, err := h.ServisRepo.DohvatiID(r.Context(), id)
	if err != nil {
		http.Error(w, "Nalog nije pronađen", http.StatusNotFound)
		return
	}

	delovi, err := h.deloviSaPotrazivanima(r.Context(), id)
	if err != nil {
		http.Error(w, "Greška pri učitavanju delova", http.StatusInternalServerError)
		return
	}

	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	var klijent *model.Klijent
	klijentNaziv := ""
	if nalog.KlijentID != nil {
		k, err := h.KlijentiRepo.DohvatiID(r.Context(), *nalog.KlijentID)
		if err == nil {
			klijent = k
			if k.NazivFirme != "" {
				klijentNaziv = k.NazivFirme
			} else {
				klijentNaziv = strings.TrimSpace(k.Ime + " " + k.Prezime)
			}
		}
	}

	tehnicarNaziv := ""
	if nalog.TehnicarID != nil {
		tehnicar, err := h.KorisniciRepo.DohvatiPoID(r.Context(), *nalog.TehnicarID)
		if err == nil {
			tehnicarNaziv = tehnicar.KorisnickoIme
		}
	}

	var ukupnoDelovi float64
	for _, d := range delovi {
		ukupnoDelovi += d.Ukupno()
	}
	var preostaloSve float64
	var imaAvans bool
	if nalog.CenaKonacna != nil {
		ukupnoSve := *nalog.CenaKonacna + ukupnoDelovi
		avans := 0.0
		if nalog.Avans != nil && *nalog.Avans > 0 {
			avans = *nalog.Avans
			imaAvans = true
		}
		preostaloSve = ukupnoSve - avans
		if preostaloSve < 0 {
			preostaloSve = 0
		}
	}

	nalogURL := qrNalogURL(r, nalog.JavniToken, podesavanja["qr_bazni_url"])
	var qrKodOtpr string
	if png, err := qrcode.Encode(nalogURL, qrcode.Medium, 160); err == nil {
		qrKodOtpr = base64.StdEncoding.EncodeToString(png)
	}

	h.renderujStandalone(w, "servis_otpremnica", PodaciOtpremnice{
		Nalog:          *nalog,
		ServisniDelovi: delovi,
		UkupnoDelovi:   ukupnoDelovi,
		PreostaloSve:   preostaloSve,
		ImaAvans:       imaAvans,
		QRKod:          qrKodOtpr,
		Klijent:        klijent,
		KlijentNaziv:   klijentNaziv,
		TehnicarNaziv:  tehnicarNaziv,
		NazivFirme:     podesavanja["naziv_firme"],
		LogoPutanja:    logoZaDokument(podesavanja),
		Podnazlov:      podesavanja["podnazlov"],
		Adresa:         podesavanja["adresa"],
		Telefon:        podesavanja["telefon"],
		PIB:            podesavanja["pib"],
		MaticniBroj:    podesavanja["maticni_broj"],
		Barkod:         barkodNaloga(nalog.BrojNaloga),
	})
}

// PodaciReversa su podaci za revers — potvrdu o prijemu uređaja na servis.
// Dokument se izdaje pri prijemu, pa nema cena.
type PodaciReversa struct {
	Nalog              model.ServisniNalog
	Klijent            *model.Klijent
	KlijentNaziv       string
	KlijentOznakaIdent string // labela identifikacije ("JMBG" / "Br. lične karte")
	KlijentBrojIdent   string // vrednost identifikacionog broja
	TehnicarNaziv      string
	QRKod              string // base64 QR ka javnoj status strani naloga
	Uslovi             string // uslovi servisa iz podešavanja (servis_uslovi)
	NazivFirme         string
	LogoPutanja        string
	Podnazlov          string
	Adresa             string
	Telefon            string
	PIB                string
	MaticniBroj        string
	Barkod             string // base64 PNG Code128 barkoda broja naloga
}

// StampaReversa renderuje revers (potvrdu o prijemu uređaja na servis)
func (h *Handler) StampaReversa(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID naloga", http.StatusBadRequest)
		return
	}

	nalog, err := h.ServisRepo.DohvatiID(r.Context(), id)
	if err != nil {
		http.Error(w, "Nalog nije pronađen", http.StatusNotFound)
		return
	}

	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	var klijent *model.Klijent
	klijentNaziv := ""
	klijentOznaka := ""
	klijentBroj := ""
	if nalog.KlijentID != nil {
		k, err := h.KlijentiRepo.DohvatiID(r.Context(), *nalog.KlijentID)
		if err == nil {
			klijent = k
			klijentNaziv = k.PunoIme()
			if k.JMBG != "" {
				klijentOznaka = k.OznakaIdentifikacije()
				klijentBroj = k.JMBG
			}
		}
	}

	tehnicarNaziv := ""
	if nalog.TehnicarID != nil {
		tehnicar, err := h.KorisniciRepo.DohvatiPoID(r.Context(), *nalog.TehnicarID)
		if err == nil {
			tehnicarNaziv = tehnicar.KorisnickoIme
		}
	}

	nalogURL := qrNalogURL(r, nalog.JavniToken, podesavanja["qr_bazni_url"])
	var qrKod string
	if png, err := qrcode.Encode(nalogURL, qrcode.Medium, 160); err == nil {
		qrKod = base64.StdEncoding.EncodeToString(png)
	}

	h.renderujStandalone(w, "servis_revers", PodaciReversa{
		Nalog:              *nalog,
		Klijent:            klijent,
		KlijentNaziv:       klijentNaziv,
		KlijentOznakaIdent: klijentOznaka,
		KlijentBrojIdent:   klijentBroj,
		TehnicarNaziv:      tehnicarNaziv,
		QRKod:              qrKod,
		Uslovi:             vrednostIliDefault(podesavanja, "servis_uslovi", podrazumevaniUsloviServisa),
		NazivFirme:         podesavanja["naziv_firme"],
		LogoPutanja:        logoZaDokument(podesavanja),
		Podnazlov:          podesavanja["podnazlov"],
		Adresa:             podesavanja["adresa"],
		Telefon:            podesavanja["telefon"],
		PIB:                podesavanja["pib"],
		MaticniBroj:        podesavanja["maticni_broj"],
		Barkod:             barkodNaloga(nalog.BrojNaloga),
	})
}

// PodaciPredracuna su podaci za predračun — procenu cene za tražene radove i delove,
// koju klijent odobrava pre popravke. Gradi se iz stavki radova i delova na nalogu.
type PodaciPredracuna struct {
	Nalog          model.ServisniNalog
	Radovi         []model.ServisniRad
	UkupnoRad      float64
	ServisniDelovi []model.ServisniDeoSaArtiklom
	UkupnoDelovi   float64
	UkupnoSve      float64
	DatumIzdavanja time.Time
	VaziDo         time.Time
	Klauzula       string
	QRKod          string
	Klijent        *model.Klijent
	KlijentNaziv   string
	TehnicarNaziv  string
	NazivFirme     string
	LogoPutanja    string
	Podnazlov      string
	Adresa         string
	Telefon        string
	PIB            string
	MaticniBroj    string
	Barkod         string // base64 PNG Code128 barkoda broja naloga
}

// StampaPredracuna renderuje predračun (procenu cene) na osnovu traženih radova i delova
func (h *Handler) StampaPredracuna(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID naloga", http.StatusBadRequest)
		return
	}

	nalog, err := h.ServisRepo.DohvatiID(r.Context(), id)
	if err != nil {
		http.Error(w, "Nalog nije pronađen", http.StatusNotFound)
		return
	}

	radovi, err := h.ServisniRadoviRepo.DohvatiZaNalog(r.Context(), id)
	if err != nil {
		http.Error(w, "Greška pri učitavanju radova", http.StatusInternalServerError)
		return
	}

	delovi, err := h.deloviSaPotrazivanima(r.Context(), id)
	if err != nil {
		http.Error(w, "Greška pri učitavanju delova", http.StatusInternalServerError)
		return
	}

	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	var klijent *model.Klijent
	klijentNaziv := ""
	if nalog.KlijentID != nil {
		k, err := h.KlijentiRepo.DohvatiID(r.Context(), *nalog.KlijentID)
		if err == nil {
			klijent = k
			klijentNaziv = k.PunoIme()
		}
	}

	tehnicarNaziv := ""
	if nalog.TehnicarID != nil {
		tehnicar, err := h.KorisniciRepo.DohvatiPoID(r.Context(), *nalog.TehnicarID)
		if err == nil {
			tehnicarNaziv = tehnicar.KorisnickoIme
		}
	}

	var ukupnoRad float64
	for _, rd := range radovi {
		ukupnoRad += rd.Ukupno()
	}
	var ukupnoDelovi float64
	for _, d := range delovi {
		ukupnoDelovi += d.Ukupno()
	}

	// rok važenja iz podešavanja (default 7 dana)
	rok := 7
	if v, err := strconv.Atoi(podesavanja["predracun_rok_dana"]); err == nil && v > 0 {
		rok = v
	}
	datumIzdavanja := time.Now()
	vaziDo := datumIzdavanja.AddDate(0, 0, rok)

	nalogURL := qrNalogURL(r, nalog.JavniToken, podesavanja["qr_bazni_url"])
	var qrKod string
	if png, err := qrcode.Encode(nalogURL, qrcode.Medium, 160); err == nil {
		qrKod = base64.StdEncoding.EncodeToString(png)
	}

	h.renderujStandalone(w, "servis_predracun", PodaciPredracuna{
		Nalog:          *nalog,
		Radovi:         radovi,
		UkupnoRad:      ukupnoRad,
		ServisniDelovi: delovi,
		UkupnoDelovi:   ukupnoDelovi,
		UkupnoSve:      ukupnoRad + ukupnoDelovi,
		DatumIzdavanja: datumIzdavanja,
		VaziDo:         vaziDo,
		Klauzula:       vrednostIliDefault(podesavanja, "servis_klauzula_predracuna", podrazumevanaKlauzulaPredracuna),
		QRKod:          qrKod,
		Klijent:        klijent,
		KlijentNaziv:   klijentNaziv,
		TehnicarNaziv:  tehnicarNaziv,
		NazivFirme:     podesavanja["naziv_firme"],
		LogoPutanja:    logoZaDokument(podesavanja),
		Podnazlov:      podesavanja["podnazlov"],
		Adresa:         podesavanja["adresa"],
		Telefon:        podesavanja["telefon"],
		PIB:            podesavanja["pib"],
		MaticniBroj:    podesavanja["maticni_broj"],
		Barkod:         barkodNaloga(nalog.BrojNaloga),
	})
}

// PodaciNalepnice su podaci za malu nalepnicu (70×40 mm) koja se lepi na uređaj
type PodaciNalepnice struct {
	Nalog         model.ServisniNalog
	KlijentNaziv  string
	Telefon       string // sirov telefon klijenta (formatira se u šablonu)
	DatumPrijema  string // kratki format, npr. "20.06."
	PredvidjenRok string // kratki format predviđenog datuma; prazno ako nije postavljen
	Barkod        string // base64 PNG Code128 barkoda sa brojem naloga (za laserski skener)
}

// barkodNaloga generiše Code128 barkod broja naloga kao base64 PNG; prazan string ako ne uspe
func barkodNaloga(broj string) string {
	kod, err := code128.Encode(broj)
	if err != nil {
		return ""
	}
	// razvuci na fiksnu širinu/visinu radi oštrog otiska na nalepnici
	skaliran, err := barcode.Scale(kod, 480, 90)
	if err != nil {
		return ""
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, skaliran); err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

// StampaNalepnice renderuje malu nalepnicu sa brojem naloga, klijentom, uređajem i QR kodom
func (h *Handler) StampaNalepnice(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID naloga", http.StatusBadRequest)
		return
	}

	nalog, err := h.ServisRepo.DohvatiID(r.Context(), id)
	if err != nil {
		http.Error(w, "Nalog nije pronađen", http.StatusNotFound)
		return
	}

	klijentNaziv := ""
	telefon := ""
	if nalog.KlijentID != nil {
		k, err := h.KlijentiRepo.DohvatiID(r.Context(), *nalog.KlijentID)
		if err == nil {
			if k.NazivFirme != "" {
				klijentNaziv = k.NazivFirme
			} else {
				klijentNaziv = strings.TrimSpace(k.Ime + " " + k.Prezime)
			}
			telefon = k.Telefon
		}
	}

	// kratki datumi za nalepnicu (npr. "20.06.")
	datumPrijema := nalog.DatumPrijema.Format("02.01.")
	predvidjenRok := ""
	if nalog.PredvidjenDatum != nil {
		predvidjenRok = nalog.PredvidjenDatum.Format("02.01.")
	}

	h.renderujStandalone(w, "servis_nalepnica", PodaciNalepnice{
		Nalog:         *nalog,
		KlijentNaziv:  klijentNaziv,
		Telefon:       telefon,
		DatumPrijema:  datumPrijema,
		PredvidjenRok: predvidjenRok,
		Barkod:        barkodNaloga(nalog.BrojNaloga),
	})
}

// PromeniStatus obrađuje POST /servis/{id}/status i menja samo status naloga
func (h *Handler) PromeniStatus(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID naloga", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}
	noviStatus := strings.TrimSpace(r.FormValue("status"))
	dozvoljenStatusi := map[string]bool{}
	for _, s := range model.SviStatusi {
		dozvoljenStatusi[s] = true
	}
	if !dozvoljenStatusi[noviStatus] {
		http.Error(w, "Nepoznat status", http.StatusBadRequest)
		return
	}

	trenutni, e := h.ServisRepo.DohvatiID(r.Context(), id)

	// „Čeka delove" je automatski status (postavlja se kad nedostaje deo) —
	// ne može se izabrati ručno ni iz jednog drugog statusa
	if noviStatus == model.StatusCekaDelove && e == nil && trenutni.Status != model.StatusCekaDelove {
		middleware.SetFlash(w, r, h.DB, "greska",
			"Status „Čeka delove\" se postavlja automatski kada nedostaje deo i ne može se izabrati ručno.")
		http.Redirect(w, r, "/servis/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
		return
	}

	// zabrana izlaska iz „Čeka delove" dok postoje potraživani delovi —
	// prvo nabaviti delove, pa obrisati iz potraživanih (ili kroz nabavku)
	if e == nil && trenutni.Status == model.StatusCekaDelove && noviStatus != model.StatusCekaDelove {
		potrazivani, err := h.ServisniPotrazivaniDeloviRepo.DohvatiZaNalog(r.Context(), id)
		if err == nil && len(potrazivani) > 0 {
			middleware.SetFlash(w, r, h.DB, "greska",
				"Nalog ne može da napusti „Čeka delove\" dok ima delova koji nedostaju. Obrišite ih iz tabele ili dopunite zalihe.")
			http.Redirect(w, r, "/servis/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
			return
		}
	}

	if err := h.ServisRepo.AzurirajStatus(r.Context(), id, noviStatus); err != nil {
		slog.Error("greška pri promeni statusa naloga", "id", id, "error", err)
		http.Error(w, "Greška pri promeni statusa", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/servis/"+strconv.FormatInt(id, 10)+"?sacuvano=1", http.StatusSeeOther)
}

// AzurirajGaranciju ažurira datum garancije na servisnom nalogu
func (h *Handler) AzurirajGaranciju(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "servis.izmeni"); !ok {
		return
	}
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID naloga", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}
	// ista logika kao u formi „Izmeni nalog" (parseFormuNaloga): popunjen datum se čuva,
	// prazno ili „bez garancije" → bez garancije. Podrazumevanu vrednost postavlja dugme
	// „Podrazumevano" eksplicitno (popuni input), pa ovde nema posebne default grane.
	var garancijaDo *time.Time
	if r.FormValue("bez_garancije") != "1" {
		if gd := strings.TrimSpace(r.FormValue("garancija_do")); gd != "" {
			t, err := time.Parse("2006-01-02", gd)
			if err != nil {
				http.Error(w, "Neispravan datum garancije", http.StatusBadRequest)
				return
			}
			garancijaDo = &t
		}
	}
	// garancija ne sme da bude pre datuma prijema
	if garancijaDo != nil {
		if nalog, e := h.ServisRepo.DohvatiID(r.Context(), id); e == nil && nalog != nil && garancijaPrePrijema(*garancijaDo, nalog.DatumPrijema) {
			http.Error(w, "Datum garancije ne može biti pre datuma prijema.", http.StatusBadRequest)
			return
		}
	}
	if err := h.ServisRepo.AzurirajGaranciju(r.Context(), id, garancijaDo); err != nil {
		slog.Error("greška pri ažuriranju garancije", "id", id, "error", err)
		http.Error(w, "Greška pri ažuriranju garancije", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/servis/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

// AzurirajGarancijaDana postavlja trajanje garancije u danima (od završetka radova);
// „bez garancije" ili prazno/0 → bez garancije.
func (h *Handler) AzurirajGarancijaDana(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "servis.izmeni"); !ok {
		return
	}
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID naloga", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}
	var dana *int
	if r.FormValue("bez_garancije") != "1" {
		if s := strings.TrimSpace(r.FormValue("garancija_dana")); s != "" {
			n, err := strconv.Atoi(s)
			if err != nil || n < 0 {
				http.Error(w, "Garancija mora biti ceo broj dana.", http.StatusBadRequest)
				return
			}
			if n > 0 {
				dana = &n
			}
		}
	}
	if err := h.ServisRepo.AzurirajGarancijaDana(r.Context(), id, dana); err != nil {
		slog.Error("greška pri ažuriranju garancije (dana)", "id", id, "error", err)
		http.Error(w, "Greška pri ažuriranju garancije", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/servis/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

// AzurirajPredvidjenDatum postavlja ručni override predviđenog datuma popravke;
// prazno polje vraća nalog na izvedeni default (prijem + rok iz podešavanja).
func (h *Handler) AzurirajPredvidjenDatum(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "servis.izmeni"); !ok {
		return
	}
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID naloga", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}
	var predvidjenDatum *time.Time
	if pd := strings.TrimSpace(r.FormValue("predvidjen_datum")); pd != "" {
		t, err := time.Parse("2006-01-02", pd)
		if err != nil {
			http.Error(w, "Neispravan predviđen datum", http.StatusBadRequest)
			return
		}
		predvidjenDatum = &t
	}
	if err := h.ServisRepo.AzurirajPredvidjenDatum(r.Context(), id, predvidjenDatum); err != nil {
		slog.Error("greška pri ažuriranju predviđenog datuma", "id", id, "error", err)
		http.Error(w, "Greška pri ažuriranju predviđenog datuma", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/servis/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

// AzurirajTehnicar menja dodeljenog servisera na nalogu; prazna vrednost → nedodeljen.
func (h *Handler) AzurirajTehnicar(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "servis.izmeni"); !ok {
		return
	}
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID naloga", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}
	var tehnicarID *int64
	if t := strings.TrimSpace(r.FormValue("tehnicar_id")); t != "" {
		v, err := strconv.ParseInt(t, 10, 64)
		if err != nil {
			http.Error(w, "Neispravan serviser", http.StatusBadRequest)
			return
		}
		tehnicarID = &v
	}
	if err := h.ServisRepo.AzurirajTehnicar(r.Context(), id, tehnicarID); err != nil {
		slog.Error("greška pri ažuriranju servisera", "id", id, "error", err)
		http.Error(w, "Greška pri ažuriranju servisera", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/servis/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

// SacuvajNapomenuKlijentu ažurira napomenu namenjenu klijentu na nalogu
func (h *Handler) SacuvajNapomenuKlijentu(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "servis.izmeni"); !ok {
		return
	}
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID naloga", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}
	tekst := strings.TrimSpace(r.FormValue("napomena_klijentu"))
	if err := h.ServisRepo.AzurirajNapomenuKlijentu(r.Context(), id, tekst); err != nil {
		slog.Error("greška pri ažuriranju napomene klijentu", "id", id, "error", err)
		http.Error(w, "Greška pri ažuriranju napomene", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/servis/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

// SacuvajNalazDijagnostike obrađuje POST /servis/{id}/nalaz-dijagnostike i čuva
// dijagnozu koju je serviser utvrdio (rezultat dijagnostike, hrani predračun)
func (h *Handler) SacuvajNalazDijagnostike(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "servis.izmeni"); !ok {
		return
	}
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID naloga", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}
	tekst := strings.TrimSpace(r.FormValue("nalaz_dijagnostike"))
	if err := h.ServisRepo.AzurirajNalazDijagnostike(r.Context(), id, tekst); err != nil {
		slog.Error("greška pri ažuriranju nalaza dijagnostike", "id", id, "error", err)
		http.Error(w, "Greška pri ažuriranju nalaza", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/servis/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

// SacuvajUradjeno obrađuje POST /servis/{id}/uradjeno i čuva tekst „šta je urađeno"
// (serviser upisuje izvršene radove tokom popravke)
func (h *Handler) SacuvajUradjeno(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "servis.izmeni"); !ok {
		return
	}
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID naloga", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}
	tekst := strings.TrimSpace(r.FormValue("uradjeno"))
	if err := h.ServisRepo.AzurirajUradjeno(r.Context(), id, tekst); err != nil {
		slog.Error("greška pri ažuriranju urađenog", "id", id, "error", err)
		http.Error(w, "Greška pri čuvanju", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/servis/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

// AzurirajCenaDijagnostike čuva cenu dijagnostike (taksa kad klijent ne prihvati popravku);
// prazno polje ili neispravna vrednost se tretira kao 0 (ne naplaćuje se)
func (h *Handler) AzurirajCenaDijagnostike(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "servis.izmeni"); !ok {
		return
	}
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID naloga", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}
	cena := 0.0
	if s := strings.TrimSpace(r.FormValue("cena_dijagnostike")); s != "" {
		v, e := strconv.ParseFloat(s, 64)
		if e != nil || v < 0 {
			http.Error(w, "Cena dijagnostike mora biti broj veći ili jednak nuli.", http.StatusBadRequest)
			return
		}
		cena = v
	}
	if err := h.ServisRepo.AzurirajCenaDijagnostike(r.Context(), id, cena); err != nil {
		slog.Error("greška pri ažuriranju cene dijagnostike", "id", id, "error", err)
		http.Error(w, "Greška pri ažuriranju cene dijagnostike", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/servis/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

// OdbijPopravku obrađuje slučaj kad klijent posle dijagnostike ne prihvati popravku:
// nalog prelazi u „Završeno" i naplaćuje se samo cena dijagnostike. Cena se uzima iz
// forme, a ako je prazna koristi se podrazumevana iz podešavanja (Podešavanja → Servis).
func (h *Handler) OdbijPopravku(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "servis.izmeni"); !ok {
		return
	}
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID naloga", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	cenaStr := strings.TrimSpace(r.FormValue("cena_dijagnostike"))
	if cenaStr == "" {
		// nema unete cene na nalogu → uzmi podrazumevanu iz podešavanja
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		cenaStr = strings.TrimSpace(podesavanja["servis_cena_dijagnostike"])
	}
	cena := 0.0
	if cenaStr != "" {
		v, e := strconv.ParseFloat(strings.ReplaceAll(cenaStr, ",", "."), 64)
		if e != nil || v < 0 {
			http.Error(w, "Cena dijagnostike mora biti broj veći ili jednak nuli.", http.StatusBadRequest)
			return
		}
		cena = v
	}

	if err := h.ServisRepo.OdbijPopravku(r.Context(), id, cena); err != nil {
		slog.Error("greška pri odbijanju popravke", "id", id, "error", err)
		http.Error(w, "Greška pri promeni statusa", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/servis/"+strconv.FormatInt(id, 10)+"?sacuvano=1", http.StatusSeeOther)
}

// PodaciJavnogStatusa su podaci za javnu status stranicu servisnog naloga
type PodaciJavnogStatusa struct {
	Nalog             model.ServisniNalog
	NazivFirme        string
	LogoPutanja       string
	Podnazlov         string
	Adresa            string
	Telefon           string
	PIB               string
	MaticniBroj       string
	Radovi            []model.ServisniRad           // ugrađeni/odobreni
	ServisniDelovi    []model.ServisniDeoSaArtiklom // ugrađeni/odobreni
	UkupnoSve         float64                       // zbir ugrađenih (radovi + delovi)
	PredlozeniRadovi  []model.ServisniRad
	PredlozeniDelovi  []model.ServisniDeoSaArtiklom
	UkupnoPredlog     float64 // zbir predloženih (radovi + delovi)
	ImaPredlog        bool
	UkupnoSaPredlogom float64 // ukupno ugrađeno + predloženo
	Odgovoreno        bool    // klijent već odgovorio (nema predloga, nije „Primljeno")
	SviStatusi        []string
}

// ServisJavniStatus prikazuje javnu status stranicu — dostupna bez prijave putem QR koda
func (h *Handler) ServisJavniStatus(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		http.NotFound(w, r)
		return
	}

	nalog, err := h.ServisRepo.DohvatiJavniToken(r.Context(), token)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// javni link se „gasi" čim klijent preuzme uređaj — posle preuzimanja status više nije dostupan
	if nalog.Status == model.StatusPreuzeto {
		http.NotFound(w, r)
		return
	}

	radovi, _ := h.ServisniRadoviRepo.DohvatiZaNalog(r.Context(), nalog.ID)
	delovi, _ := h.deloviSaPotrazivanima(r.Context(), nalog.ID)

	// razdvajamo ugrađene/odobrene od predloženih (čekaju odluku klijenta)
	var ugrRadovi, predRadovi []model.ServisniRad
	var ukupnoSve, ukupnoPredlog float64
	for _, rd := range radovi {
		if rd.Predlozeno {
			predRadovi = append(predRadovi, rd)
			ukupnoPredlog += rd.Ukupno()
		} else {
			ugrRadovi = append(ugrRadovi, rd)
			ukupnoSve += rd.Ukupno()
		}
	}
	var ugrDelovi, predDelovi []model.ServisniDeoSaArtiklom
	for _, d := range delovi {
		if d.Predlozeno {
			predDelovi = append(predDelovi, d)
			ukupnoPredlog += d.Ukupno()
		} else {
			ugrDelovi = append(ugrDelovi, d)
			ukupnoSve += d.Ukupno()
		}
	}

	podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)

	h.renderujStandalone(w, "servis_status_javni", PodaciJavnogStatusa{
		Nalog:             *nalog,
		NazivFirme:        podesavanja["naziv_firme"],
		LogoPutanja:       logoZaDokument(podesavanja),
		Podnazlov:         podesavanja["podnazlov"],
		Adresa:            podesavanja["adresa"],
		Telefon:           podesavanja["telefon"],
		PIB:               podesavanja["pib"],
		MaticniBroj:       podesavanja["maticni_broj"],
		Radovi:            ugrRadovi,
		ServisniDelovi:    ugrDelovi,
		UkupnoSve:         ukupnoSve,
		PredlozeniRadovi:  predRadovi,
		PredlozeniDelovi:  predDelovi,
		UkupnoPredlog:     ukupnoPredlog,
		ImaPredlog:        len(predRadovi) > 0 || len(predDelovi) > 0,
		UkupnoSaPredlogom: ukupnoSve + ukupnoPredlog,
		Odgovoreno:        nalog.OdlukaKlijenta != "" || (!(len(predRadovi) > 0 || len(predDelovi) > 0) && nalog.Status != model.StatusPrimljeno),
		SviStatusi:        model.SviStatusi,
	})
}

// qrNalogURL konstruiše URL za QR kod vodeći računa o reverse proxy-ju.
// Ako je u podešavanjima zadata fiksna adresa (bazniURL, npr. http://192.168.1.25:3000),
// koristi se ona — korisno da QR radi sa telefona kad se programu pristupa preko „localhost".
// Inače se host i šema čitaju iz zahteva (X-Forwarded-Proto pokriva nginx/Caddy/Traefik).
func qrNalogURL(r *http.Request, token, bazniURL string) string {
	if b := strings.TrimRight(strings.TrimSpace(bazniURL), "/"); b != "" {
		return b + "/status/" + token
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return scheme + "://" + r.Host + "/status/" + token
}

// ServisJavniPrihvati obrađuje klijentovo prihvatanje predloženih stavki.
// Sve predložene stavke (radovi i delovi) prelaze u ugrađene (predlozeno 1→0).
func (h *Handler) ServisJavniPrihvati(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		http.NotFound(w, r)
		return
	}

	nalog, err := h.ServisRepo.DohvatiJavniToken(r.Context(), token)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if nalog.Status == model.StatusPreuzeto {
		http.NotFound(w, r)
		return
	}

	// odluka je već doneta — ne dozvoljavamo ponovno glasanje
	if nalog.OdlukaKlijenta != "" {
		http.Redirect(w, r, "/status/"+token, http.StatusSeeOther)
		return
	}

	if err := h.ServisniRadoviRepo.PrihvatiPredlozene(r.Context(), nalog.ID); err != nil {
		slog.Error("greška pri prihvatanju predloženih radova", "error", err, "nalog_id", nalog.ID)
		http.Error(w, "Greška pri prihvatanju predloga.", http.StatusInternalServerError)
		return
	}
	if err := h.ServisniDeloviRepo.PrihvatiPredlozene(r.Context(), nalog.ID); err != nil {
		slog.Error("greška pri prihvatanju predloženih delova", "error", err, "nalog_id", nalog.ID)
		http.Error(w, "Greška pri prihvatanju predloga.", http.StatusInternalServerError)
		return
	}

	komentar := strings.TrimSpace(r.FormValue("komentar"))
	if err := h.ServisRepo.SacuvajOdlukuKlijenta(r.Context(), nalog.ID, "prihvaceno", komentar); err != nil {
		slog.Error("greška pri čuvanju odluke klijenta", "error", err, "nalog_id", nalog.ID)
	}

	http.Redirect(w, r, "/status/"+token, http.StatusSeeOther)
}

// ObrisiKomentarKlijenta briše poruku klijenta sa naloga
func (h *Handler) ObrisiKomentarKlijenta(w http.ResponseWriter, r *http.Request) {
	_, ok := h.zahtevajDozvolu(w, r, "servis.izmeni")
	if !ok {
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := h.ServisRepo.AzurirajKomentarKlijenta(r.Context(), id, ""); err != nil {
		slog.Error("greška pri brisanju komentara klijenta", "error", err, "nalog_id", id)
		http.Error(w, "Greška pri brisanju komentara.", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/servis/"+chi.URLParam(r, "id"), http.StatusSeeOther)
}

// ServisJavniOdbij beleži da je klijent video predloge ali ih ne prihvata.
// Ne briše stavke — serviser ih ručno uklanja ako želi.
func (h *Handler) ServisJavniOdbij(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		http.NotFound(w, r)
		return
	}

	nalog, err := h.ServisRepo.DohvatiJavniToken(r.Context(), token)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if nalog.Status == model.StatusPreuzeto {
		http.NotFound(w, r)
		return
	}

	// odluka je već doneta — ne dozvoljavamo ponovno glasanje
	if nalog.OdlukaKlijenta != "" {
		http.Redirect(w, r, "/status/"+token, http.StatusSeeOther)
		return
	}

	// brišemo predložene stavke — klijent ih je odbio
	if err := h.ServisniRadoviRepo.ObrisiPredlozene(r.Context(), nalog.ID); err != nil {
		slog.Error("greška pri brisanju predloženih radova", "error", err, "nalog_id", nalog.ID)
	}
	if err := h.ServisniDeloviRepo.ObrisiPredlozene(r.Context(), nalog.ID); err != nil {
		slog.Error("greška pri brisanju predloženih delova", "error", err, "nalog_id", nalog.ID)
	}

	komentar := strings.TrimSpace(r.FormValue("komentar"))
	if err := h.ServisRepo.SacuvajOdlukuKlijenta(r.Context(), nalog.ID, "odbijeno", komentar); err != nil {
		slog.Error("greška pri čuvanju odluke klijenta", "error", err, "nalog_id", nalog.ID)
	}

	http.Redirect(w, r, "/status/"+token, http.StatusSeeOther)
}
