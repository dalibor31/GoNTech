package handler

import (
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ntech/internal/db"
	"ntech/internal/db/sqlite"
	"ntech/internal/model"

	"github.com/go-chi/chi/v5"
)

// PodaciNabavki su podaci za stranicu sa listom nabavki
type PodaciNabavki struct {
	model.PodaciStranice
	Nabavke  []model.NabavkaSaDetaljem
	Sacuvano bool
	Obrisan  bool
}

// PodaciFormeNabavke su podaci za formu unosa nove nabavke
type PodaciFormeNabavke struct {
	model.PodaciStranice
	Artikli     []model.ArtikalSaKategorijom
	ArtikliJSON template.JS // JSON niz artikala za Alpine.js — bezbedan za umetanje u <script>
	Dobavljaci  []model.Dobavljac
	Kategorije  []model.Kategorija // za dropdown u modalu novog artikla
	Marza       string             // podrazumevana marža (%) za kalkulaciju
	PdvObveznik bool               // da li firma obračunava PDV (utiče na prodajnu cenu u kalkulaciji)
	Danas       string             // format "2006-01-02" — default za datum računa
	Greska      string
}

// PodaciDetaljiNabavke su podaci za pregled jedne nabavke sa stavkama
type PodaciDetaljiNabavke struct {
	model.PodaciStranice
	Nabavka        model.Nabavka
	Stavke         []model.StavkaSaArtiklom
	Troskovi       []model.NabavkaTrosak
	UkupanTrosak   float64
	DobavljacNaziv string
	Sacuvano       bool
	Greska         string
}

// artikalUJSON pretvara listu artikala u template.JS vrednost bezbednu za umetanje u <script> tag
func artikalUJSON(artikli []model.ArtikalSaKategorijom, vezeDobavljaca map[int64][]int64) template.JS {
	type stavka struct {
		ID              int64    `json:"id"`
		Naziv           string   `json:"naziv"`
		PdvStopa        float64  `json:"pdv_stopa"`
		NabavnaCena     float64  `json:"nabavna_cena"`     // poslednja nabavna cena — predlog za Cena/kom
		Marza           *float64 `json:"marza"`            // marža artikla; null = nije postavljeno
		KategorijaMarza *float64 `json:"kategorija_marza"` // marža kategorije; fallback ako artikal nema
		Dobavljaci      []int64  `json:"dobavljaci"`       // ID-jevi dobavljača koji isporučuju artikal
	}
	lista := make([]stavka, 0, len(artikli))
	for _, a := range artikli {
		dob := vezeDobavljaca[a.ID]
		if dob == nil {
			dob = []int64{}
		}
		lista = append(lista, stavka{
			ID: a.ID, Naziv: a.Naziv, PdvStopa: a.PdvStopa,
			NabavnaCena: a.NabavnaCena,
			Marza:       a.Marza, KategorijaMarza: a.KategorijaMarza,
			Dobavljaci: dob,
		})
	}
	b, _ := json.Marshal(lista)
	return template.JS(b)
}

// Nabavke renderuje listu svih nabavki
func (h *Handler) Nabavke(w http.ResponseWriter, r *http.Request) {
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	nabavke, err := h.NabavkeRepo.Lista(r.Context())
	if err != nil {
		http.Error(w, "Greška pri učitavanju nabavki", http.StatusInternalServerError)
		return
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "nabavke"
	ps.NaslovStranice = "Nabavke"
	podaci := PodaciNabavki{
		PodaciStranice: ps,
		Nabavke:        nabavke,
		Sacuvano:       r.URL.Query().Get("sacuvano") == "1",
		Obrisan:        r.URL.Query().Get("obrisan") == "1",
	}

	h.renderujTemplate(w, "nabavke", podaci)
}

// NovaNabavka prikazuje formu za unos nove nabavke
func (h *Handler) NovaNabavka(w http.ResponseWriter, r *http.Request) {
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

	dobavljaci, err := h.DobavljaciRepo.Lista(r.Context(), "")
	if err != nil {
		http.Error(w, "Greška pri učitavanju dobavljača", http.StatusInternalServerError)
		return
	}

	kategorije, err := h.KategorijeRepo.Lista(r.Context())
	if err != nil {
		http.Error(w, "Greška pri učitavanju kategorija", http.StatusInternalServerError)
		return
	}

	veze, _ := h.Artikli.SveDobavljaceArtikala(r.Context())

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "nabavke"
	ps.NaslovStranice = "Nova nabavka"
	h.renderujFormuNabavke(w, PodaciFormeNabavke{
		PodaciStranice: ps,
		Artikli:        artikli,
		ArtikliJSON:    artikalUJSON(artikli, veze),
		Dobavljaci:     dobavljaci,
		Kategorije:     kategorije,
		Marza:          vrednostIliDefault(podesavanja, "kalkulacija_marza", "20"),
		PdvObveznik:    h.modulUkljucen(r.Context(), "pdv"),
		Danas:          time.Now().Format("2006-01-02"),
	})
}

// SacuvajNabavku prima POST formu, parsira stavke i upisuje nabavku u bazu
func (h *Handler) SacuvajNabavku(w http.ResponseWriter, r *http.Request) {
	k, ok := h.zahtevajDozvolu(w, r, "nabavka.dodaj")
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	nabavka, stavke, troskovi, greska := parseFormuNabavke(r)
	if greska == "" && h.modulUkljucen(r.Context(), "pdv") {
		switch {
		case nabavka.DobavljacID == nil:
			greska = "PDV evidencija je uključena — dobavljač je obavezan da bi KPR imao ispravan PIB."
		case nabavka.BrojRacuna == "":
			greska = "PDV evidencija je uključena — broj računa dobavljača je obavezan za KPR."
		}
	}
	if greska != "" {
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		artikli, _ := h.Artikli.Lista(r.Context(), db.ArtikalFilter{})
		dobavljaci, _ := h.DobavljaciRepo.Lista(r.Context(), "")
		kategorije, _ := h.KategorijeRepo.Lista(r.Context())
		veze, _ := h.Artikli.SveDobavljaceArtikala(r.Context())
		ps := h.popuniPodaciStranice(r, podesavanja)
		ps.Stranica = "nabavke"
		ps.NaslovStranice = "Nova nabavka"
		h.renderujFormuNabavke(w, PodaciFormeNabavke{
			PodaciStranice: ps,
			Artikli:        artikli,
			ArtikliJSON:    artikalUJSON(artikli, veze),
			Dobavljaci:     dobavljaci,
			Kategorije:     kategorije,
			Marza:          vrednostIliDefault(podesavanja, "kalkulacija_marza", "20"),
			PdvObveznik:    h.modulUkljucen(r.Context(), "pdv"),
			Danas:          time.Now().Format("2006-01-02"),
			Greska:         greska,
		})
		return
	}

	id, err := h.NabavkeRepo.Kreiraj(r.Context(), &nabavka, stavke, troskovi, &k.ID)
	if err != nil {
		http.Error(w, "Greška pri čuvanju nabavke", http.StatusInternalServerError)
		return
	}

	// automatski zavedi u KPR ako je firma PDV obveznik; PDV se izvodi iz stope artikla
	if h.modulUkljucen(r.Context(), "pdv") {
		var stavkePdv []model.NabavkaStavkaPdv
		for _, s := range stavke {
			var stopa float64
			if a, e := h.Artikli.DohvatiID(r.Context(), s.ArtikalID); e == nil {
				stopa = a.PdvStopa
			}
			stavkePdv = append(stavkePdv, model.NabavkaStavkaPdv{
				Osnovica: float64(s.Kolicina) * s.CenaPoKomadu,
				PdvStopa: stopa,
			})
		}
		naziv, pib, mesto := "Nepoznat dobavljač", "", ""
		if nabavka.DobavljacID != nil {
			if d, e := h.DobavljaciRepo.DohvatiID(r.Context(), *nabavka.DobavljacID); e == nil {
				naziv, pib, mesto = d.Naziv, d.PIB, d.Mesto
			}
		}
		nabavka.ID = id
		nabavka.Datum = time.Now()
		kpr := model.KprIzNabavke(nabavka, naziv, pib, mesto, stavkePdv)
		if _, e := h.PdvKprRepo.Kreiraj(r.Context(), &kpr); e != nil {
			slog.Error("auto-upis u KPR nije uspeo", "nabavka_id", id, "error", e)
		}
	}

	// kalkulacija: ažuriraj prodajnu cenu artikla iz forme + nivelacioni trag.
	// nabavna cena je već ponderisana prosečna (postavlja je Kreiraj) — ne preglašava se.
	// prodajna[] je paralelni niz uz stavke (isti redosled kao artikal_id[]).
	prodajne := r.Form["prodajna[]"]
	var korisnikID *int64
	if k != nil {
		korisnikID = &k.ID
	}
	for i, s := range stavke {
		if i >= len(prodajne) {
			break
		}
		prodajna, e := strconv.ParseFloat(strings.TrimSpace(prodajne[i]), 64)
		if e != nil || prodajna <= 0 {
			continue // prazno/nula ne sme da pregazi postojeću cenu
		}
		// stara prodajna i tekuća (ponderisana) nabavna — nabavnu zadržavamo
		var staraProdajna, nabavna float64
		if a, e := h.Artikli.DohvatiID(r.Context(), s.ArtikalID); e == nil {
			staraProdajna = a.ProdajnaCena
			nabavna = a.NabavnaCena
		}
		if e := h.Artikli.AzurirajCene(r.Context(), s.ArtikalID, nabavna, prodajna); e != nil {
			slog.Error("kalkulacija: ažuriranje cena nije uspelo", "artikal_id", s.ArtikalID, "error", e)
			continue
		}
		if razlika := prodajna - staraProdajna; razlika > 0.005 || razlika < -0.005 {
			if _, e := h.NivelacijaRepo.Kreiraj(r.Context(), &model.Nivelacija{
				ArtikalID:  s.ArtikalID,
				StaraCena:  staraProdajna,
				NovaCena:   prodajna,
				Izvor:      "kalkulacija",
				KorisnikID: korisnikID,
			}); e != nil {
				slog.Error("kalkulacija: nivelacija nije upisana", "artikal_id", s.ArtikalID, "error", e)
			}
		}
	}

	// auto-veza artikal–dobavljač: svaki nabavljeni artikal se veže za dobavljača nabavke
	if nabavka.DobavljacID != nil {
		for _, s := range stavke {
			if e := h.Artikli.PoveziDobavljaca(r.Context(), s.ArtikalID, *nabavka.DobavljacID); e != nil {
				slog.Error("auto-veza dobavljača nije upisana", "artikal_id", s.ArtikalID, "error", e)
			}
		}
	}

	// nakon povećanja stanja: počisti potraživane delove i vrati otključane naloge na Primljeno
	for _, s := range stavke {
		otkljucani, err := h.ServisniPotrazivaniDeloviRepo.ProveriIPocistiZaArtikal(r.Context(), s.ArtikalID)
		if err != nil {
			slog.Error("provera potraživanih delova nije uspela", "artikal_id", s.ArtikalID, "error", err)
			continue
		}
		for _, nalogID := range otkljucani {
			if err := h.ServisRepo.AzurirajStatus(r.Context(), nalogID, model.StatusPrimljeno); err != nil {
				slog.Error("automatski reset statusa naloga nije uspeo", "nalog_id", nalogID, "error", err)
			}
		}
	}

	http.Redirect(w, r, "/nabavke/"+strconv.FormatInt(id, 10)+"?sacuvano=1", http.StatusSeeOther)
}

// DetaljiNabavke prikazuje pregled jedne nabavke sa svim stavkama
func (h *Handler) DetaljiNabavke(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID nabavke", http.StatusBadRequest)
		return
	}

	nabavka, err := h.NabavkeRepo.DohvatiID(r.Context(), id)
	if err != nil {
		slog.Error("DetaljiNabavke: DohvatiID greška", "id", id, "error", err)
		http.Error(w, "Nabavka nije pronađena", http.StatusNotFound)
		return
	}

	stavke, err := h.NabavkeRepo.DohvatiStavke(r.Context(), id)
	if err != nil {
		http.Error(w, "Greška pri učitavanju stavki", http.StatusInternalServerError)
		return
	}

	troskovi, err := h.NabavkeRepo.DohvatiTroskove(r.Context(), id)
	if err != nil {
		http.Error(w, "Greška pri učitavanju troškova", http.StatusInternalServerError)
		return
	}

	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	// naziv dobavljača dohvatamo samo ako nabavka ima dobavljača
	dobavljacNaziv := ""
	if nabavka.DobavljacID != nil {
		dobavljac, err := h.DobavljaciRepo.DohvatiID(r.Context(), *nabavka.DobavljacID)
		if err == nil {
			dobavljacNaziv = dobavljac.Naziv
		}
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "nabavke"
	ps.NaslovStranice = "Detalji nabavke"
	var ukupanTrosak float64
	for _, t := range troskovi {
		ukupanTrosak += t.Iznos
	}
	var greska string
	if r.URL.Query().Get("greska") == "storno" {
		greska = "Storniranje nije uspelo. Nabavka je možda već stornirana."
	}
	podaci := PodaciDetaljiNabavke{
		PodaciStranice: ps,
		Nabavka:        *nabavka,
		Stavke:         stavke,
		Troskovi:       troskovi,
		UkupanTrosak:   ukupanTrosak,
		DobavljacNaziv: dobavljacNaziv,
		Sacuvano:       r.URL.Query().Get("sacuvano") == "1",
		Greska:         greska,
	}

	h.renderujTemplate(w, "nabavka_detalji", podaci)
}

// StornoNabavke prima POST zahtev i stornira nabavku po ID-u (umesto fizičkog brisanja)
func (h *Handler) StornoNabavke(w http.ResponseWriter, r *http.Request) {
	k, ok := h.zahtevajDozvolu(w, r, "nabavka.storno")
	if !ok {
		return
	}
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID nabavke", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}
	razlog := strings.TrimSpace(r.FormValue("razlog"))

	if err := h.NabavkeRepo.Storno(r.Context(), id, razlog, &k.ID); err != nil {
		slog.Error("greška pri storniranju nabavke", "error", err)
		http.Redirect(w, r, "/nabavke/"+strconv.FormatInt(id, 10)+"?greska=storno", http.StatusSeeOther)
		return
	}
	// stornirana nabavka ne ulazi u PDV — ukloni vezani auto-KPR zapis
	if err := h.PdvKprRepo.ObrisiPoIzvoru(r.Context(), "nabavka", id); err != nil {
		slog.Error("brisanje vezanog KPR zapisa nije uspelo", "nabavka_id", id, "error", err)
	}

	http.Redirect(w, r, "/nabavke/"+strconv.FormatInt(id, 10)+"?sacuvano=1", http.StatusSeeOther)
}

// parseFormuNabavke čita zaglavlje, stavke i zavisne troškove iz HTTP forme
// i vraća model, stavke, troškove i eventualnu grešku
func parseFormuNabavke(r *http.Request) (model.Nabavka, []model.StavkaNabavke, []model.NabavkaTrosak, string) {
	var nabavka model.Nabavka

	// opcioni dobavljač
	if dobavljacIDStr := r.FormValue("dobavljac_id"); dobavljacIDStr != "" {
		id, err := strconv.ParseInt(dobavljacIDStr, 10, 64)
		if err == nil {
			nabavka.DobavljacID = &id
		}
	}
	nabavka.Napomena = strings.TrimSpace(r.FormValue("napomena"))
	nabavka.BrojRacuna = strings.TrimSpace(r.FormValue("broj_racuna"))

	if datStr := strings.TrimSpace(r.FormValue("datum_racuna")); datStr != "" {
		if t, e := time.Parse("2006-01-02", datStr); e == nil {
			nabavka.DatumRacuna = &t
		}
	}

	if pdvStr := strings.TrimSpace(r.FormValue("pdv_iznos")); pdvStr != "" {
		if v, e := strconv.ParseFloat(pdvStr, 64); e == nil && v > 0 {
			nabavka.PdvIznos = v
		}
	}

	// paralelni nizovi stavki
	artikalIDovi := r.Form["artikal_id[]"]
	kolicine := r.Form["kolicina[]"]
	cene := r.Form["cena_po_komadu[]"]

	var stavke []model.StavkaNabavke
	for i := range artikalIDovi {
		// preskoči red koji korisnik nije popunio
		artikalIDStr := strings.TrimSpace(artikalIDovi[i])
		if artikalIDStr == "" {
			continue
		}
		artikalID, err := strconv.ParseInt(artikalIDStr, 10, 64)
		if err != nil || artikalID <= 0 {
			continue
		}
		var kolicina int
		if i < len(kolicine) {
			kolicina, _ = strconv.Atoi(strings.TrimSpace(kolicine[i]))
		}
		if kolicina <= 0 {
			kolicina = 1
		}
		var cena float64
		if i < len(cene) {
			cena, _ = strconv.ParseFloat(strings.TrimSpace(cene[i]), 64)
		}
		stavke = append(stavke, model.StavkaNabavke{
			ArtikalID:    artikalID,
			Kolicina:     kolicina,
			CenaPoKomadu: cena,
		})
	}

	if len(stavke) == 0 {
		return nabavka, nil, nil, "Nabavka mora imati najmanje jednu stavku."
	}

	// zavisni troškovi (opcioni, paralelni nizovi); prazni redovi se preskaču
	naziviT := r.Form["trosak_naziv[]"]
	iznosiT := r.Form["trosak_iznos[]"]
	var troskovi []model.NabavkaTrosak
	for i := range naziviT {
		naziv := strings.TrimSpace(naziviT[i])
		if naziv == "" {
			continue
		}
		var iznos float64
		if i < len(iznosiT) {
			iznos, _ = strconv.ParseFloat(strings.TrimSpace(iznosiT[i]), 64)
		}
		if iznos <= 0 {
			continue
		}
		troskovi = append(troskovi, model.NabavkaTrosak{Naziv: naziv, Iznos: iznos})
	}

	// metod raspodele je bitan samo ako ima troškova; default je po vrednosti
	if len(troskovi) > 0 {
		metod := r.FormValue("metod_raspodele")
		if metod != "kolicina" {
			metod = "vrednost"
		}
		nabavka.MetodRaspodele = metod
	}

	return nabavka, stavke, troskovi, ""
}

// renderujFormuNabavke renderuje HTML šablon forme za unos nove nabavke
func (h *Handler) renderujFormuNabavke(w http.ResponseWriter, podaci PodaciFormeNabavke) {
	h.renderujTemplate(w, "nabavka_forma", podaci)
}
