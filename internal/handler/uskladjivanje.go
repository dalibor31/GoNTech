package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	appdb "ntech/internal/db"
	"ntech/internal/middleware"
	"ntech/internal/model"
)

// kandidatUskladjivanja opisuje jedan nalog koji se razmatra za automatski backfill
// KIR/KPO upisa — koristi se samo za detekciju sumnjivih duplikata (double-submit),
// ne za sam upis.
type kandidatUskladjivanja struct {
	ID        int64
	KlijentID *int64
	Iznos     float64
	Datum     time.Time
	ImaUpis   bool
}

// sumnjiviDuplikati vraća ID-eve naloga BEZ upisa koje automatski backfill treba da
// PRESKOČI — jer postoji drugi (bliski) nalog istog klijenta i iznosa u roku od minuta
// koji već ima upis, ili je stariji i takođe čeka na upis. Ovo je znak dvostrukog
// slanja forme (double-submit bag na naplati), ne dva odvojena prometa — automatska
// dopuna bi inače duplirala prihod. Prosleđuje se CEO skup naloga (i sa i bez upisa)
// radi poređenja.
func sumnjiviDuplikati(svi []kandidatUskladjivanja) map[int64]bool {
	const prozor = 60 * time.Second
	sumnjivi := make(map[int64]bool)
	for _, a := range svi {
		if a.ImaUpis {
			continue
		}
		for _, b := range svi {
			if a.ID == b.ID {
				continue
			}
			if !istKlijent(a.KlijentID, b.KlijentID) || a.Iznos != b.Iznos {
				continue
			}
			razlika := a.Datum.Sub(b.Datum)
			if razlika < 0 {
				razlika = -razlika
			}
			if razlika > prozor {
				continue
			}
			if b.ImaUpis || b.ID < a.ID {
				sumnjivi[a.ID] = true
				break
			}
		}
	}
	return sumnjivi
}

// kirKandidatiProdaje vraća sve B2B prodajne naloge (nezavisno od toga da li već imaju
// KIR upis) sa mapom postojećih upisa i kandidatima za detekciju duplikata. Koristi se
// i za backfill (KirBackfillProdaje) i za brojanje praznina na dashboard-u.
func (h *Handler) kirKandidatiProdaje(ctx context.Context) ([]model.ProdajniNalogSaDetaljem, map[int64]bool, []kandidatUskladjivanja, error) {
	svi, err := h.ProdajaRepo.Lista(ctx, appdb.ProdajaFilter{})
	if err != nil {
		return nil, nil, nil, err
	}
	// stornirani nalozi se nikad ne razmatraju za KIR upis — ni za backfill ni za
	// brojanje praznina — jer stornirana prodaja ne predstavlja oporeziv promet
	var nalozi []model.ProdajniNalogSaDetaljem
	for _, nd := range svi {
		if !nd.Stornirano {
			nalozi = append(nalozi, nd)
		}
	}
	postojiMap := make(map[int64]bool, len(nalozi))
	var kandidati []kandidatUskladjivanja
	for _, nd := range nalozi {
		if nd.KlijentID == nil {
			continue
		}
		postoji, _ := h.PdvKirRepo.PostojiZaIzvor(ctx, "prodaja", nd.ID)
		postojiMap[nd.ID] = postoji
		kandidati = append(kandidati, kandidatUskladjivanja{
			ID: nd.ID, KlijentID: nd.KlijentID, Iznos: nd.Ukupno, Datum: nd.Datum, ImaUpis: postoji,
		})
	}
	return nalozi, postojiMap, kandidati, nil
}

// kpoKandidatiProdaje vraća sve nestornirane prodajne naloge sa mapom postojećih KPO
// upisa i kandidatima za detekciju duplikata.
func (h *Handler) kpoKandidatiProdaje(ctx context.Context) ([]model.ProdajniNalogSaDetaljem, map[int64]bool, []kandidatUskladjivanja, error) {
	nalozi, err := h.ProdajaRepo.Lista(ctx, appdb.ProdajaFilter{})
	if err != nil {
		return nil, nil, nil, err
	}
	postojiMap := make(map[int64]bool, len(nalozi))
	var kandidati []kandidatUskladjivanja
	for _, nd := range nalozi {
		if nd.Stornirano {
			continue
		}
		postoji, _ := h.KpoRepo.PostojiZaIzvor(ctx, "prodaja", nd.ID)
		postojiMap[nd.ID] = postoji
		kandidati = append(kandidati, kandidatUskladjivanja{
			ID: nd.ID, KlijentID: nd.KlijentID, Iznos: nd.Ukupno, Datum: nd.Datum, ImaUpis: postoji,
		})
	}
	return nalozi, postojiMap, kandidati, nil
}

// kpoKandidatiServisa vraća sve naplaćene i preuzete servisne naloge sa mapom
// postojećih KPO upisa, kandidatima za detekciju duplikata, i brojem naloga koji su
// izostavljeni jer nisu (još) naplaćeni/preuzeti.
func (h *Handler) kpoKandidatiServisa(ctx context.Context) (naplaceni []model.ServisniNalogSaKlijentom, postojiMap map[int64]bool, kandidati []kandidatUskladjivanja, nenaplaceniBroj int, err error) {
	servisNalozi, err := h.ServisRepo.Lista(ctx, "", "")
	if err != nil {
		return nil, nil, nil, 0, err
	}
	naplaceni = make([]model.ServisniNalogSaKlijentom, 0, len(servisNalozi))
	for _, sn := range servisNalozi {
		if sn.Status == model.StatusPreuzeto && sn.Naplaceno != 0 && !sn.Stornirano {
			naplaceni = append(naplaceni, sn)
		} else {
			nenaplaceniBroj++
		}
	}
	postojiMap = make(map[int64]bool, len(naplaceni))
	for _, sn := range naplaceni {
		postoji, _ := h.KpoRepo.PostojiZaIzvor(ctx, "servis", sn.ID)
		postojiMap[sn.ID] = postoji
		datum := sn.DatumPrijema
		if sn.DatumZavrsetka != nil {
			datum = *sn.DatumZavrsetka
		}
		kandidati = append(kandidati, kandidatUskladjivanja{
			ID: sn.ID, KlijentID: sn.KlijentID, Iznos: sn.Naplaceno, Datum: datum, ImaUpis: postoji,
		})
	}
	return naplaceni, postojiMap, kandidati, nenaplaceniBroj, nil
}

// kirKandidatiServisa vraća sve preuzete servisne naloge sa identifikovanim kupcem
// (KlijentID != nil, isti kriterijum kao u Prodaji — B2B/identifikovan kupac) sa
// mapom postojećih KIR upisa i kandidatima za detekciju duplikata.
func (h *Handler) kirKandidatiServisa(ctx context.Context) ([]model.ServisniNalogSaKlijentom, map[int64]bool, []kandidatUskladjivanja, error) {
	svi, err := h.ServisRepo.Lista(ctx, "", "")
	if err != nil {
		return nil, nil, nil, err
	}
	var kandidatNalozi []model.ServisniNalogSaKlijentom
	for _, sn := range svi {
		if sn.Status == model.StatusPreuzeto && !sn.Stornirano && !sn.PopravkaOdbijena && sn.KlijentID != nil {
			kandidatNalozi = append(kandidatNalozi, sn)
		}
	}
	postojiMap := make(map[int64]bool, len(kandidatNalozi))
	var kandidati []kandidatUskladjivanja
	for _, sn := range kandidatNalozi {
		postoji, _ := h.PdvKirRepo.PostojiZaIzvor(ctx, "servis", sn.ID)
		postojiMap[sn.ID] = postoji
		datum := sn.DatumPrijema
		if sn.DatumZavrsetka != nil {
			datum = *sn.DatumZavrsetka
		}
		kandidati = append(kandidati, kandidatUskladjivanja{
			ID: sn.ID, KlijentID: sn.KlijentID, Iznos: sn.Naplaceno, Datum: datum, ImaUpis: postoji,
		})
	}
	return kandidatNalozi, postojiMap, kandidati, nil
}

// PraznineKnjigovodstva broji naloge (prodaja + servis) koji čekaju KIR/KPO/fiskalni
// upis prema TRENUTNO aktivnim modulima firme, i naloge koji su sumnjivi duplikati
// (double-submit) pa ih automatski backfill preskače.
func (h *Handler) PraznineKnjigovodstva(ctx context.Context) (model.PrazninaKnjigovodstva, error) {
	var p model.PrazninaKnjigovodstva
	var kandidatiP []kandidatUskladjivanja

	if h.modulUkljucen(ctx, "pdv") {
		_, postojiMapP, kp, err := h.kirKandidatiProdaje(ctx)
		if err != nil {
			return p, err
		}
		kandidatiP = kp
		_, postojiMapS, kandidatiS, err := h.kirKandidatiServisa(ctx)
		if err != nil {
			return p, err
		}
		duplikatiP := sumnjiviDuplikati(kandidatiP)
		duplikatiS := sumnjiviDuplikati(kandidatiS)
		for id, postoji := range postojiMapP {
			if postoji {
				continue
			}
			if duplikatiP[id] {
				p.SumnjiviDupli++
				continue
			}
			p.BezKirProdajaID = append(p.BezKirProdajaID, id)
		}
		for id, postoji := range postojiMapS {
			if postoji {
				continue
			}
			if duplikatiS[id] {
				p.SumnjiviDupli++
				continue
			}
			p.BezKirServisID = append(p.BezKirServisID, id)
		}
		p.BezKirProdaja = len(p.BezKirProdajaID)
		p.BezKirServis = len(p.BezKirServisID)
	}

	if h.modulUkljucen(ctx, "kpo") {
		_, postojiMapP, kp, err := h.kpoKandidatiProdaje(ctx)
		if err != nil {
			return p, err
		}
		kandidatiP = kp
		_, postojiMapS, kandidatiS, _, err := h.kpoKandidatiServisa(ctx)
		if err != nil {
			return p, err
		}
		duplikatiP := sumnjiviDuplikati(kandidatiP)
		duplikatiS := sumnjiviDuplikati(kandidatiS)
		for id, postoji := range postojiMapP {
			if postoji {
				continue
			}
			if duplikatiP[id] {
				p.SumnjiviDupli++
				continue
			}
			p.BezKpoProdajaID = append(p.BezKpoProdajaID, id)
		}
		for id, postoji := range postojiMapS {
			if postoji {
				continue
			}
			if duplikatiS[id] {
				p.SumnjiviDupli++
				continue
			}
			p.BezKpoServisID = append(p.BezKpoServisID, id)
		}
		p.BezKpoProdaja = len(p.BezKpoProdajaID)
		p.BezKpoServis = len(p.BezKpoServisID)
	}

	if h.modulUkljucen(ctx, "fiskalizacija") {
		if m, err := h.FiskalRepo.ProdajeBezFiskalnog(ctx); err == nil {
			for id := range m {
				p.BezFiskalnogProdajaID = append(p.BezFiskalnogProdajaID, id)
			}
			p.BezFiskalnogProdaja = len(p.BezFiskalnogProdajaID)
		}
		if m, err := h.FiskalRepo.ServisiBezFiskalnog(ctx); err == nil {
			for id := range m {
				p.BezFiskalnogServisID = append(p.BezFiskalnogServisID, id)
			}
			p.BezFiskalnogServis = len(p.BezFiskalnogServisID)
		}

		// sumnjivi duplikati za fiskal — isti kandidati kao za KPO/PDV,
		// ako su već učitani; ako ne, učitavamo iz prodaje (samo brojimo,
		// preskaču se u automatskoj batch fiskalizaciji)
		if len(kandidatiP) > 0 {
			dupli := sumnjiviDuplikati(kandidatiP)
			for id := range dupli {
				p.SumnjiviFiskalID = append(p.SumnjiviFiskalID, id)
			}
		}

		if idsP, idsS, err := h.stornoBezRefunda(ctx); err == nil {
			p.BezRefundaProdajaID = idsP
			p.BezRefundaServisID = idsS
			p.BezRefundaProdaja = len(idsP)
			p.BezRefundaServis = len(idsS)
		}
	}

	return p, nil
}

// stornoBezRefunda vraća ID-jeve storniranih prodajnih i servisnih naloga čiji je
// originalni fiskalni račun i dalje neoznačen kao storniran — znači da je storno u
// bazi prošao, ali fiskalni refund ka ESIR/L-PFR uređaju NIJE uspeo (best-effort
// poziv je pao).
func (h *Handler) stornoBezRefunda(ctx context.Context) (prodajaIDs, servisIDs []int64, err error) {
	redoviP, err := h.DB.QueryContext(ctx, `
		SELECT DISTINCT pn.id FROM prodajni_nalozi pn
			JOIN fiskalni_racuni fr ON fr.prodaja_id = pn.id
				AND fr.tip_racuna = 'Normal' AND fr.tip_transakcije = 'Sale' AND fr.storniran = 0
			WHERE pn.stornirano = 1
	`)
	if err != nil {
		return nil, nil, err
	}
	defer redoviP.Close()
	for redoviP.Next() {
		var id int64
		if err := redoviP.Scan(&id); err != nil {
			return nil, nil, err
		}
		prodajaIDs = append(prodajaIDs, id)
	}
	if err := redoviP.Err(); err != nil {
		return nil, nil, err
	}

	// tip_racuna='Normal' je bitno: servis sa avansom ima i Advance/Sale zapis koji
	// se NIKAD ne stornira pri storno servisa (samo konačan Normal/Sale) — bez ovog
	// filtera bi JOIN dao dva reda za isti servis (duplikat u listi na dashboard-u)
	redoviS, err := h.DB.QueryContext(ctx, `
		SELECT DISTINCT sn.id FROM servisni_nalozi sn
			JOIN fiskalni_racuni fr ON fr.servis_id = sn.id
				AND fr.tip_racuna = 'Normal' AND fr.tip_transakcije = 'Sale' AND fr.storniran = 0
			WHERE sn.stornirano = 1
	`)
	if err != nil {
		return nil, nil, err
	}
	defer redoviS.Close()
	for redoviS.Next() {
		var id int64
		if err := redoviS.Scan(&id); err != nil {
			return nil, nil, err
		}
		servisIDs = append(servisIDs, id)
	}
	if err := redoviS.Err(); err != nil {
		return nil, nil, err
	}
	return prodajaIDs, servisIDs, nil
}

func istKlijent(a, b *int64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// BatchFiskalizacija pokreće fiskalizaciju svih prodaja i servisa koji nemaju
// fiskalni račun. Best-effort: ako neka fiskalizacija padne, nastavlja se.
func (h *Handler) BatchFiskalizacija(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	klijent := h.fiskalKlijent(ctx)
	if klijent == nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Fiskalni servis nije dostupan. Proverite vezu sa ESIR/PFR.")
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}

	izdato := 0
	if m, err := h.FiskalRepo.ProdajeBezFiskalnog(ctx); err == nil {
		for id := range m {
			h.fiskalizujProdaju(ctx, id, klijent, 0)
			izdato++
		}
	}
	if m, err := h.FiskalRepo.ServisiBezFiskalnog(ctx); err == nil {
		for id := range m {
			h.fiskalizujServis(ctx, id, klijent, "Gotovina", 0, 0)
			izdato++
		}
	}

	if izdato == 0 {
		middleware.SetFlash(w, r, h.DB, "uspeh", "Svi nalozi već imaju fiskalni račun.")
	} else {
		middleware.SetFlash(w, r, h.DB, "uspeh", "Pokrenuta fiskalizacija za "+strconv.Itoa(izdato)+" naloga.")
	}
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}
