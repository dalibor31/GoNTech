package handler

import (
	"context"
	"time"

	appdb "ntech/internal/db"
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
	nalozi, err := h.ProdajaRepo.Lista(ctx, appdb.ProdajaFilter{})
	if err != nil {
		return nil, nil, nil, err
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
		if sn.Status == model.StatusPreuzeto && sn.Naplaceno != 0 {
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

// PraznineKnjigovodstva broji naloge (prodaja + servis) koji čekaju KIR/KPO/fiskalni
// upis prema TRENUTNO aktivnim modulima firme, i naloge koji su sumnjivi duplikati
// (double-submit) pa ih automatski backfill preskače.
func (h *Handler) PraznineKnjigovodstva(ctx context.Context) (model.PrazninaKnjigovodstva, error) {
	var p model.PrazninaKnjigovodstva

	if h.modulUkljucen(ctx, "pdv") {
		_, postojiMap, kandidati, err := h.kirKandidatiProdaje(ctx)
		if err != nil {
			return p, err
		}
		duplikati := sumnjiviDuplikati(kandidati)
		for id, postoji := range postojiMap {
			if postoji {
				continue
			}
			if duplikati[id] {
				p.SumnjiviDupli++
				continue
			}
			p.BezKir++
		}
	}

	if h.modulUkljucen(ctx, "kpo") {
		_, postojiMapP, kandidatiP, err := h.kpoKandidatiProdaje(ctx)
		if err != nil {
			return p, err
		}
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
			p.BezKpo++
		}
		for id, postoji := range postojiMapS {
			if postoji {
				continue
			}
			if duplikatiS[id] {
				p.SumnjiviDupli++
				continue
			}
			p.BezKpo++
		}
	}

	if h.modulUkljucen(ctx, "fiskalizacija") {
		if m, err := h.FiskalRepo.ProdajeBezFiskalnog(ctx); err == nil {
			p.BezFiskalnog += len(m)
		}
		if m, err := h.FiskalRepo.ServisiBezFiskalnog(ctx); err == nil {
			p.BezFiskalnog += len(m)
		}
		if n, err := h.stornoBezRefunda(ctx); err == nil {
			p.BezRefunda = n
		}
	}

	return p, nil
}

// stornoBezRefunda broji stornirane prodajne naloge čiji je originalni fiskalni
// račun i dalje neoznačen kao storniran — znači da je storno u bazi prošao, ali
// fiskalni refund ka ESIR/L-PFR uređaju NIJE uspeo (best-effort poziv je pao).
// Servis nema storno funkciju — obuhvaćena je samo prodaja.
func (h *Handler) stornoBezRefunda(ctx context.Context) (int, error) {
	var n int
	err := h.DB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM prodajni_nalozi pn
		JOIN fiskalni_racuni fr ON fr.prodaja_id = pn.id
			AND fr.tip_transakcije = 'Sale' AND fr.storniran = 0
		WHERE pn.stornirano = 1
	`).Scan(&n)
	return n, err
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
