package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ntech/internal/config"
	appdb "ntech/internal/db"
	"ntech/internal/db/sqlite"
	"ntech/internal/fiskal"
	"ntech/internal/middleware"
	"ntech/internal/model"

	"github.com/go-chi/chi/v5"
)

// PodaciProdaje su podaci za stranicu sa listom prodajnih naloga
type PodaciProdaje struct {
	model.PodaciStranice
	Nalozi        []model.ProdajniNalogSaDetaljem
	Sacuvano      bool
	Obrisan       bool
	Pretraga      string
	NemaFiskalnog map[int64]bool // ID-evi naloga bez izdatog fiskalnog računa
}

// PodaciFormeProdaje su podaci za formu unosa nove prodaje
type PodaciFormeProdaje struct {
	model.PodaciStranice
	Artikli     []model.ArtikalSaKategorijom
	ArtikliJSON template.JS
	Klijenti    []model.Klijent
	Greska      string
}

// PodaciDetaljiProdaje su podaci za pregled jedne prodaje sa stavkama
type PodaciDetaljiProdaje struct {
	model.PodaciStranice
	Nalog         model.ProdajniNalog
	Stavke        []model.StavkaProdajeSaArtiklom
	KlijentNaziv  string
	FiskalniRacun *model.FiskalniRacun // nil ako nije fiskalizovano
	FiskalGreska  bool                 // true: fiskalizacija aktivna, nalog nije storniran, ali nema fiskalnog računa
	Sacuvano      bool
}

// PodaciStampeProdaje su podaci za stranicu za štampanje priznanice
type PodaciStampeProdaje struct {
	Nalog        model.ProdajniNalog
	Stavke       []model.StavkaProdajeSaArtiklom
	KlijentNaziv string
	Moduli       map[string]bool
	UkupnoBezPdv float64
	NazivFirme   string
	Podnazlov    string
	Adresa       string
	Telefon      string
	PIB          string
}

// artikalUJSONSaCenom pretvara listu artikala u template.JS vrednost sa prodajnom cenom, PDV stopom, kategorijom i stanjem
func artikalUJSONSaCenom(artikli []model.ArtikalSaKategorijom) template.JS {
	type stavka struct {
		ID              int64   `json:"id"`
		Naziv           string  `json:"naziv"`
		Sifra           string  `json:"sifra"`
		Barkod          string  `json:"barkod"`
		Cena            float64 `json:"cena"`
		CenaSaPdv       float64 `json:"cena_sa_pdv"`
		PdvStopa        float64 `json:"pdv_stopa"`
		Kolicina        int     `json:"kolicina"`
		KategorijaNaziv string  `json:"kategorija_naziv"`
	}
	lista := make([]stavka, 0, len(artikli))
	for _, a := range artikli {
		lista = append(lista, stavka{
			ID:              a.ID,
			Naziv:           a.Naziv,
			Sifra:           a.Sifra,
			Barkod:          a.Barkod,
			Cena:            a.ProdajnaCena,
			CenaSaPdv:       a.CenaSaPdv,
			PdvStopa:        a.PdvStopa,
			Kolicina:        a.Kolicina,
			KategorijaNaziv: a.KategorijaNaziv,
		})
	}
	b, _ := json.Marshal(lista)
	return template.JS(b)
}

// Prodaja renderuje listu svih prodajnih naloga
func (h *Handler) Prodaja(w http.ResponseWriter, r *http.Request) {
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	pretraga := strings.TrimSpace(r.URL.Query().Get("pretraga"))

	nalozi, err := h.ProdajaRepo.Lista(r.Context(), pretraga)
	if err != nil {
		http.Error(w, "Greška pri učitavanju prodaje", http.StatusInternalServerError)
		return
	}

	var nemaFiskalnog map[int64]bool
	if h.modulUkljucen(r.Context(), config.ModulFiskalizacija) {
		nemaFiskalnog, _ = h.FiskalRepo.ProdajeBezFiskalnog(r.Context())
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "prodaja"
	ps.NaslovStranice = "Prodaja"
	podaci := PodaciProdaje{
		PodaciStranice: ps,
		Nalozi:         nalozi,
		Sacuvano:       r.URL.Query().Get("sacuvano") == "1",
		Obrisan:        r.URL.Query().Get("obrisan") == "1",
		Pretraga:       pretraga,
		NemaFiskalnog:  nemaFiskalnog,
	}

	h.renderujTemplate(w, "prodaja", podaci)
}

// NovaProdaja prikazuje formu za unos novog prodajnog naloga
func (h *Handler) NovaProdaja(w http.ResponseWriter, r *http.Request) {
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	artikli, err := h.Artikli.Lista(r.Context(), appdb.ArtikalFilter{})
	if err != nil {
		http.Error(w, "Greška pri učitavanju artikala", http.StatusInternalServerError)
		return
	}

	klijenti, err := h.KlijentiRepo.Lista(r.Context(), "")
	if err != nil {
		http.Error(w, "Greška pri učitavanju klijenata", http.StatusInternalServerError)
		return
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "prodaja"
	ps.NaslovStranice = "Nova prodaja"
	h.renderujFormuProdaje(w, PodaciFormeProdaje{
		PodaciStranice: ps,
		Artikli:        artikli,
		ArtikliJSON:    artikalUJSONSaCenom(artikli),
		Klijenti:       klijenti,
	})
}

// SacuvajProdaju prima POST formu, parsira stavke i upisuje prodajni nalog u bazu
func (h *Handler) SacuvajProdaju(w http.ResponseWriter, r *http.Request) {
	k, ok := h.zahtevajDozvolu(w, r, "prodaja.dodaj")
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	nalog, stavke, greska := parseFormuProdaje(r)

	renderujGresku := func(poruka string) {
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		artikli, _ := h.Artikli.Lista(r.Context(), appdb.ArtikalFilter{})
		klijenti, _ := h.KlijentiRepo.Lista(r.Context(), "")
		ps := h.popuniPodaciStranice(r, podesavanja)
		ps.Stranica = "prodaja"
		ps.NaslovStranice = "Nova prodaja"
		h.renderujFormuProdaje(w, PodaciFormeProdaje{
			PodaciStranice: ps,
			Artikli:        artikli,
			ArtikliJSON:    artikalUJSONSaCenom(artikli),
			Klijenti:       klijenti,
			Greska:         poruka,
		})
	}

	if greska != "" {
		renderujGresku(greska)
		return
	}

	// za firme koje nisu PDV obveznici prisilno nulliraj PDV stopu (odbrana od klijentske greške)
	if !h.modulUkljucen(r.Context(), "pdv") {
		for i := range stavke {
			stavke[i].PdvStopa = 0
		}
	}

	brojNaloga, err := h.ProdajaRepo.SledeciBroj(r.Context())
	if err != nil {
		slog.Error("greška pri generisanju broja naloga", "error", err)
		renderujGresku("Greška pri generisanju broja naloga.")
		return
	}

	nalog.BrojNaloga = brojNaloga
	nalog.Datum = time.Now()

	var ukupno float64
	for _, s := range stavke {
		cenaPoslePopusta := s.CenaPoKomadu
		if s.PopustProcenat > 0 {
			cenaPoslePopusta = cenaPoslePopusta * (1 - s.PopustProcenat/100)
		}
		ukupno += float64(s.Kolicina) * cenaPoslePopusta
	}
	nalog.Ukupno = ukupno

	id, err := h.ProdajaRepo.Kreiraj(r.Context(), &nalog, stavke, &k.ID)
	if err != nil {
		var errStanje *appdb.ErrNedovoljnoKolicine
		if errors.As(err, &errStanje) {
			renderujGresku(errStanje.Error())
		} else {
			slog.Error("greška pri čuvanju prodaje", "error", err)
			renderujGresku("Greška pri čuvanju prodajnog naloga.")
		}
		return
	}

	// automatski zavedi u KIR ako je firma PDV obveznik i prodaja je na klijenta (B2B faktura).
	// Maloprodaja građanima (bez klijenta) ide zbirno preko fiskalizacije (Faza 3) — preskače se.
	if nalog.KlijentID != nil && h.modulUkljucen(r.Context(), "pdv") {
		if klijent, e := h.KlijentiRepo.DohvatiID(r.Context(), *nalog.KlijentID); e == nil {
			nalog.ID = id
			pib := klijent.PIB
			if klijent.Tip != "pravno" {
				pib = klijent.JMBG
			}
			kir := model.KirIzProdaje(nalog, stavke, klijent.PunoIme(), pib, klijent.Mesto)
			if _, e := h.PdvKirRepo.Kreiraj(r.Context(), &kir); e != nil {
				slog.Error("auto-upis u KIR nije uspeo", "prodaja_id", id, "error", e)
			}
		}
	}

	// Fiskalizacija — ako je modul uključen (best-effort: prodaja ostaje validna i bez fiskalizacije)
	if h.modulUkljucen(r.Context(), config.ModulFiskalizacija) {
		if klijent := h.fiskalKlijent(); klijent != nil {
			h.fiskalizujProdaju(r.Context(), id, klijent)
		}
	}

	// auto-KPO: svaki potvrđeni nalog ide u KPO ako je modul aktivan
	if h.modulUkljucen(r.Context(), "kpo") {
		kpoZ := model.KpoZapis{
			DatumPrometa:  nalog.Datum,
			BrojDokumenta: nalog.BrojNaloga,
			Opis:          fmt.Sprintf("Prodaja %s", nalog.BrojNaloga),
			Prihod:        nalog.Ukupno,
			NacinPlacanja: nalog.NacinPlacanja,
			Izvor:         "prodaja",
			IzvorID:       &id,
		}
		if _, e := h.KpoRepo.Kreiraj(r.Context(), &kpoZ); e != nil {
			slog.Error("auto-upis u KPO nije uspeo", "prodaja_id", id, "error", e)
		}
	}

	http.Redirect(w, r, "/prodaja/"+strconv.FormatInt(id, 10)+"?sacuvano=1", http.StatusSeeOther)
}

// DetaljiProdaje prikazuje pregled jednog prodajnog naloga sa svim stavkama
func (h *Handler) DetaljiProdaje(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID naloga", http.StatusBadRequest)
		return
	}

	nalog, err := h.ProdajaRepo.DohvatiID(r.Context(), id)
	if err != nil {
		http.Error(w, "Nalog nije pronađen", http.StatusNotFound)
		return
	}

	stavke, err := h.ProdajaRepo.DohvatiStavke(r.Context(), id)
	if err != nil {
		http.Error(w, "Greška pri učitavanju stavki", http.StatusInternalServerError)
		return
	}

	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	klijentNaziv := ""
	if nalog.KlijentID != nil {
		klijent, err := h.KlijentiRepo.DohvatiID(r.Context(), *nalog.KlijentID)
		if err == nil {
			if klijent.NazivFirme != "" {
				klijentNaziv = klijent.NazivFirme
			} else {
				klijentNaziv = strings.TrimSpace(klijent.Ime + " " + klijent.Prezime)
			}
		}
	}

	// učitaj fiskalni račun ako postoji
	fr, _ := h.FiskalRepo.DohvatiPoProdaji(r.Context(), id)

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "prodaja"
	ps.NaslovStranice = "Detalji prodaje"
	podaci := PodaciDetaljiProdaje{
		PodaciStranice: ps,
		Nalog:          *nalog,
		Stavke:         stavke,
		KlijentNaziv:   klijentNaziv,
		FiskalniRacun:  fr,
		FiskalGreska:   h.modulUkljucen(r.Context(), config.ModulFiskalizacija) && !nalog.Stornirano && fr == nil,
		Sacuvano:       r.URL.Query().Get("sacuvano") == "1",
	}

	h.renderujTemplate(w, "prodaja_detalji", podaci)
}

// StampaProdaje renderuje print-friendly stranicu za dati prodajni nalog
func (h *Handler) StampaProdaje(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID naloga", http.StatusBadRequest)
		return
	}

	nalog, err := h.ProdajaRepo.DohvatiID(r.Context(), id)
	if err != nil {
		http.Error(w, "Nalog nije pronađen", http.StatusNotFound)
		return
	}

	stavke, err := h.ProdajaRepo.DohvatiStavke(r.Context(), id)
	if err != nil {
		http.Error(w, "Greška pri učitavanju stavki", http.StatusInternalServerError)
		return
	}

	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	klijentNaziv := ""
	if nalog.KlijentID != nil {
		klijent, err := h.KlijentiRepo.DohvatiID(r.Context(), *nalog.KlijentID)
		if err == nil {
			if klijent.NazivFirme != "" {
				klijentNaziv = klijent.NazivFirme
			} else {
				klijentNaziv = strings.TrimSpace(klijent.Ime + " " + klijent.Prezime)
			}
		}
	}

	moduli := config.SviModuli(podesavanja)
	var ukupnoBezPdv float64
	for _, s := range stavke {
		ukupnoBezPdv += s.CenaBezPdv * float64(s.Kolicina)
	}

	podaci := PodaciStampeProdaje{
		Nalog:        *nalog,
		Stavke:       stavke,
		KlijentNaziv: klijentNaziv,
		Moduli:       moduli,
		UkupnoBezPdv: ukupnoBezPdv,
		NazivFirme:   podesavanja["naziv_firme"],
		Podnazlov:    podesavanja["podnazlov"],
		Adresa:       podesavanja["adresa"],
		Telefon:      podesavanja["telefon"],
		PIB:          podesavanja["pib"],
	}

	h.renderujStandalone(w, "prodaja_stampa", podaci)
}

// ObrisiProdaju prima POST zahtev i stornira nalog umesto fizičkog brisanja.
// Jednom izdat račun ne sme da nestane — umesto brisanja koristi se storno.
func (h *Handler) ObrisiProdaju(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "prodaja.storno"); !ok {
		return
	}
	k := middleware.KorisnikIzKonteksta(r.Context())
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID naloga", http.StatusBadRequest)
		return
	}

	if err := h.ProdajaRepo.Storno(r.Context(), id, "administrativno uklanjanje", &k.ID); err != nil {
		slog.Error("greška pri uklanjanju naloga", "error", err)
	}
	// ukloni vezani auto-KIR zapis (ako ga je ova prodaja kreirala)
	if err := h.PdvKirRepo.ObrisiPoIzvoru(r.Context(), "prodaja", id); err != nil {
		slog.Error("brisanje vezanog KIR zapisa nije uspelo", "prodaja_id", id, "error", err)
	}

	http.Redirect(w, r, "/prodaja?obrisan=1", http.StatusSeeOther)
}

// parseFormuProdaje čita zaglavlje i stavke iz HTTP forme i vraća model i eventualnu grešku
func parseFormuProdaje(r *http.Request) (model.ProdajniNalog, []model.StavkaProdaje, string) {
	var nalog model.ProdajniNalog

	if klijentIDStr := r.FormValue("klijent_id"); klijentIDStr != "" {
		id, err := strconv.ParseInt(klijentIDStr, 10, 64)
		if err == nil {
			nalog.KlijentID = &id
		}
	}
	nalog.Napomena = strings.TrimSpace(r.FormValue("napomena"))
	nalog.NacinPlacanja = r.FormValue("nacin_placanja")
	if nalog.NacinPlacanja != "gotovina" && nalog.NacinPlacanja != "kartica" && nalog.NacinPlacanja != "prenos" {
		nalog.NacinPlacanja = "gotovina"
	}

	artikalIDovi := r.Form["artikal_id[]"]
	kolicine := r.Form["kolicina[]"]
	cene := r.Form["cena_po_komadu[]"]
	pdvStope := r.Form["pdv_stopa[]"]
	popusti := r.Form["popust[]"]

	if len(artikalIDovi) == 0 {
		return nalog, nil, "Prodaja mora imati najmanje jednu stavku."
	}

	if len(artikalIDovi) != len(kolicine) || len(artikalIDovi) != len(cene) {
		return nalog, nil, "Greška u podacima forme — broj stavki nije ispravan."
	}

	var stavke []model.StavkaProdaje
	for i := range artikalIDovi {
		artikalID, err := strconv.ParseInt(strings.TrimSpace(artikalIDovi[i]), 10, 64)
		if err != nil || artikalID <= 0 {
			return nalog, nil, "Neispravan artikal u stavci."
		}

		kolicina, err := strconv.Atoi(strings.TrimSpace(kolicine[i]))
		if err != nil || kolicina <= 0 {
			return nalog, nil, "Količina mora biti pozitivan broj."
		}

		cena, err := strconv.ParseFloat(strings.TrimSpace(cene[i]), 64)
		if err != nil || cena < 0 {
			return nalog, nil, "Cena mora biti pozitivan broj."
		}

		var pdvStopa float64
		if i < len(pdvStope) {
			pdvStopa, _ = strconv.ParseFloat(strings.TrimSpace(pdvStope[i]), 64)
		}

		var popust float64
		if i < len(popusti) {
			popust, _ = strconv.ParseFloat(strings.TrimSpace(popusti[i]), 64)
		}

		stavke = append(stavke, model.StavkaProdaje{
			ArtikalID:      artikalID,
			Kolicina:       kolicina,
			CenaPoKomadu:   cena,
			PdvStopa:       pdvStopa,
			PopustProcenat: popust,
		})
	}

	return nalog, stavke, ""
}

// StornoProdaje stornira prodajni nalog: vraća artikle na stanje i označava nalog kao storniran
func (h *Handler) StornoProdaje(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if !h.DozvoleRepo.ImaDozvolu(r.Context(), k.Uloga, "prodaja.storno") {
		http.Error(w, "Nemate dozvolu za storniranje prodaje.", http.StatusForbidden)
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
	razlog := strings.TrimSpace(r.FormValue("razlog"))
	if err := h.ProdajaRepo.Storno(r.Context(), id, razlog, &k.ID); err != nil {
		slog.Error("greška pri storniranju naloga", "error", err)
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri storniranju. Možda je nalog već storniran.")
		http.Redirect(w, r, "/prodaja/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
		return
	}
	// stornirana prodaja ne ulazi u PDV — ukloni vezani auto-KIR zapis
	if err := h.PdvKirRepo.ObrisiPoIzvoru(r.Context(), "prodaja", id); err != nil {
		slog.Error("brisanje vezanog KIR zapisa nije uspelo", "prodaja_id", id, "error", err)
	}

	// fiskalni refund — best-effort (storno ostaje validan i bez uspešnog refunda)
	if h.modulUkljucen(r.Context(), config.ModulFiskalizacija) {
		if fr, _ := h.FiskalRepo.DohvatiPoProdaji(r.Context(), id); fr != nil && !fr.Storniran {
			if fk := h.fiskalKlijent(); fk != nil {
				if nalogFisk, e := h.ProdajaRepo.DohvatiID(r.Context(), id); e == nil {
					if stavkeFisk, e2 := h.ProdajaRepo.DohvatiStavke(r.Context(), id); e2 == nil {
						kasirFisk, _ := sqlite.DohvatiPodesavanje(r.Context(), h.DB, "pfr_kasir")
						if kasirFisk == "" {
							kasirFisk = "NTech"
						}
						var pibFisk, jmbgFisk string
						if nalogFisk.KlijentID != nil {
							if kk, e3 := h.KlijentiRepo.DohvatiID(r.Context(), *nalogFisk.KlijentID); e3 == nil {
								pibFisk, jmbgFisk = kk.PIB, kk.JMBG
							}
						}
						zahtevFisk := fiskal.NapraviRefundZahtev(*nalogFisk, stavkeFisk, pibFisk, jmbgFisk, kasirFisk, fr.PfrBroj)
						if odgFisk, errFisk := fk.IzdajRacun(r.Context(), zahtevFisk); errFisk != nil {
							slog.Error("fiskalni refund nije uspeo", "prodaja_id", id, "error", errFisk)
						} else {
							poreskeJSON, _ := json.Marshal(odgFisk.TaxItems)
							siroviJSON, _ := json.Marshal(odgFisk)
							refundFr := &model.FiskalniRacun{
								ProdajaID:         id,
								TipRacuna:         "Normal",
								TipTransakcije:    "Refund",
								PfrBroj:           odgFisk.InvoiceNumber,
								PfrVreme:          odgFisk.SdcDateTime,
								Brojac:            odgFisk.InvoiceCounter,
								EkstenzijaBrojaca: odgFisk.InvoiceCounterExtension,
								UrlVerifikacija:   odgFisk.VerificationURL,
								QRKod:             odgFisk.VerificationQRCode,
								PoreskeStavke:     string(poreskeJSON),
								UkupnoZaNaplatu:   odgFisk.TotalAmount,
								UkupanPorez:       odgFisk.TotalTax,
								SiroviOdgovor:     string(siroviJSON),
								Potpisao:          odgFisk.SignedBy,
								Zatrazio:          odgFisk.RequestedBy,
								Poruka:            odgFisk.Messages,
							}
							if _, errFisk = h.FiskalRepo.Kreiraj(r.Context(), refundFr); errFisk != nil {
								slog.Error("čuvanje fiskalnog refunda nije uspelo", "prodaja_id", id, "error", errFisk)
							} else {
								_ = h.FiskalRepo.OznačiKaoStorniran(r.Context(), fr.ID)
							}
						}
					}
				}
			}
		}
	}

	// brisanje KPO zapisa na storno
	if h.modulUkljucen(r.Context(), "kpo") {
		if e := h.KpoRepo.ObrisiPoIzvoru(r.Context(), "prodaja", id); e != nil {
			slog.Error("brisanje vezanog KPO zapisa nije uspelo", "prodaja_id", id, "error", e)
		}
	}

	http.Redirect(w, r, "/prodaja/"+strconv.FormatInt(id, 10)+"?sacuvano=1", http.StatusSeeOther)
}

// fiskalizujProdaju šalje fiskalni zahtev za prodajni nalog. Best-effort: greške
// se loguju, ne zaustavljaju tok pozivaoca (kreiranje prodaje ili ručni retry).
func (h *Handler) fiskalizujProdaju(ctx context.Context, prodajaID int64, klijent *fiskal.Klijent) {
	nalog, err := h.ProdajaRepo.DohvatiID(ctx, prodajaID)
	if err != nil {
		slog.Error("fiskalizujProdaju: nije pronađen nalog", "prodaja_id", prodajaID, "error", err)
		return
	}
	stavke, err := h.ProdajaRepo.DohvatiStavke(ctx, prodajaID)
	if err != nil {
		slog.Error("fiskalizujProdaju: greška pri dohvatanju stavki", "prodaja_id", prodajaID, "error", err)
		return
	}

	kasir, _ := sqlite.DohvatiPodesavanje(ctx, h.DB, "pfr_kasir")
	if kasir == "" {
		kasir = "NTech"
	}

	var pib, jmbg string
	if nalog.KlijentID != nil {
		if kk, e := h.KlijentiRepo.DohvatiID(ctx, *nalog.KlijentID); e == nil {
			pib, jmbg = kk.PIB, kk.JMBG
		}
	}

	zahtev := fiskal.NapraviZahtev(*nalog, stavke, pib, jmbg, kasir)

	odgovor, errFisk := klijent.IzdajRacun(ctx, zahtev)
	if errFisk != nil {
		slog.Error("fiskalizacija prodaje nije uspela", "prodaja_id", prodajaID, "error", errFisk)
		return
	}

	poreskeJSON, _ := json.Marshal(odgovor.TaxItems)
	siroviJSON, _ := json.Marshal(odgovor)
	fr := &model.FiskalniRacun{
		ProdajaID:         prodajaID,
		TipRacuna:         "Normal",
		TipTransakcije:    "Sale",
		PfrBroj:           odgovor.InvoiceNumber,
		PfrVreme:          odgovor.SdcDateTime,
		Brojac:            odgovor.InvoiceCounter,
		EkstenzijaBrojaca: odgovor.InvoiceCounterExtension,
		UrlVerifikacija:   odgovor.VerificationURL,
		QRKod:             odgovor.VerificationQRCode,
		PoreskeStavke:     string(poreskeJSON),
		UkupnoZaNaplatu:   odgovor.TotalAmount,
		UkupanPorez:       odgovor.TotalTax,
		SiroviOdgovor:     string(siroviJSON),
		Potpisao:          odgovor.SignedBy,
		Zatrazio:          odgovor.RequestedBy,
		Poruka:            odgovor.Messages,
	}
	if _, err := h.FiskalRepo.Kreiraj(ctx, fr); err != nil {
		slog.Error("greška pri čuvanju fiskalnog računa", "prodaja_id", prodajaID, "error", err)
	}
}

// RetryFiskalizacijaProdaje pokušava ponovo da izda fiskalni račun za prodajni
// nalog čija je prethodna fiskalizacija pala (ESIR/PFR nedostupan pri kreiranju).
func (h *Handler) RetryFiskalizacijaProdaje(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "prodaja.dodaj"); !ok {
		return
	}
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Neispravan ID naloga", http.StatusBadRequest)
		return
	}

	// zaštita od duplikata — ako fiskalni račun već postoji, ne šalji ponovo
	if fr, _ := h.FiskalRepo.DohvatiPoProdaji(r.Context(), id); fr != nil {
		middleware.SetFlash(w, r, h.DB, "uspeh", "Fiskalni račun već postoji.")
		http.Redirect(w, r, "/prodaja/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
		return
	}

	nalog, err := h.ProdajaRepo.DohvatiID(r.Context(), id)
	if err != nil || nalog.Stornirano {
		http.Error(w, "Nalog ne postoji ili je stornirano", http.StatusBadRequest)
		return
	}

	klijent := h.fiskalKlijent()
	if klijent == nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Fiskalni servis nije dostupan. Proverite vezu sa ESIR/PFR.")
		http.Redirect(w, r, "/prodaja/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
		return
	}

	h.fiskalizujProdaju(r.Context(), id, klijent)

	if fr, _ := h.FiskalRepo.DohvatiPoProdaji(r.Context(), id); fr != nil {
		middleware.SetFlash(w, r, h.DB, "uspeh", "Fiskalni račun izdat.")
	} else {
		middleware.SetFlash(w, r, h.DB, "greska", "Fiskalizacija nije uspela. Proverite vezu sa ESIR/PFR i pokušajte ponovo.")
	}
	http.Redirect(w, r, "/prodaja/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

// renderujFormuProdaje renderuje HTML šablon forme za unos nove prodaje
func (h *Handler) renderujFormuProdaje(w http.ResponseWriter, podaci PodaciFormeProdaje) {
	h.renderujTemplate(w, "prodaja_forma", podaci)
}
