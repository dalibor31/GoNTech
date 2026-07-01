package fiskal

import (
	"math"
	"strings"

	"ntech/internal/model"
)

// OznakaPDV mapira NTech PDV stopu na ćiriličnu oznaku koju Fisk/Teron očekuje.
// ⚠️ 0% → "А" (neobveznici PDV-a), NE "Е" (Е = 10% posebna stopa).
func OznakaPDV(stopa float64) string {
	switch {
	case stopa >= 19.9 && stopa <= 20.1:
		return "Ж" // opšta stopa 20%
	case stopa >= 9.9 && stopa <= 10.1:
		return "Ђ" // snižena stopa 10%
	case stopa >= -0.01 && stopa <= 0.01:
		return "А" // oslobođen / neobveznik PDV-a
	default:
		return "А" // fallback — ne rizikujemo pogrešnu oznaku
	}
}

// BrutoCena konvertuje neto cenu u bruto (sa PDV-om) za zadatu stopu.
// Teron zahteva bruto iznose u unitPrice i totalAmount.
// Formula: bruto = neto * (1 + stopa/100), zaokruženo na 2 decimale.
func BrutoCena(neto, stopa float64) float64 {
	return math.Round(neto*(1+stopa/100)*100) / 100
}

// TipPlacanja mapira NTech način plaćanja na Teron enumeraciju. Poredi bez obzira
// na velika/mala slova jer Prodaja čuva nazive malim slovima ("gotovina", "kartica",
// "prenos"), a Servis velikim ("Gotovina", "Kartica", "Virman").
func TipPlacanja(nacin string) string {
	switch strings.ToLower(nacin) {
	case "gotovina":
		return "Cash"
	case "kartica":
		return "Card"
	case "virman", "račun", "racun", "prenos":
		return "WireTransfer"
	case "ček", "cek":
		return "Check"
	case "vaučer", "vaucer":
		return "Voucher"
	default:
		return "Other"
	}
}

// NapraviZahtev konvertuje prodajni nalog i stavke u Teron InvoiceRequest.
// VAŽNO: sve cene se konvertuju iz neto u bruto pre slanja (BrutoCena).
// pib/jmbg se koriste za buyerId identifikaciju kupca.
func NapraviZahtev(
	nalog model.ProdajniNalog,
	stavke []model.StavkaProdajeSaArtiklom,
	pib, jmbg, kasir string,
) InvoiceRequest {
	items := make([]InvoiceItem, 0, len(stavke))
	ukupanIznos := 0.0

	for _, s := range stavke {
		stopa := 20.0 // podrazumevano 20% ako stopa nije eksplicitno postavljena
		if s.PdvStopa > 0 {
			stopa = s.PdvStopa
		}
		// CenaBezPdv iz baze je već diskontovana (popust primenjen pri kreiranju naloga);
		// popust se ponovo primenjuje samo u fallback grani kad CenaBezPdv nije popunjena.
		netoCena := s.CenaBezPdv
		if netoCena == 0 {
			netoCena = s.CenaPoKomadu
			if s.PopustProcenat > 0 {
				netoCena = math.Round(netoCena*(1-s.PopustProcenat/100)*100) / 100
			}
		}
		brutoCena := BrutoCena(netoCena, stopa)
		brutoTotal := math.Round(brutoCena*float64(s.Kolicina)*100) / 100

		items = append(items, InvoiceItem{
			Name:        s.ArtikalNaziv,
			Labels:      []string{OznakaPDV(stopa)},
			TotalAmount: brutoTotal,
			UnitPrice:   brutoCena,
			Quantity:    float64(s.Kolicina),
		})
		ukupanIznos += brutoTotal
	}

	// Ukupno za plaćanje je uvek suma bruto stavki (neto iz naloga bi bilo pogrešno)
	ukupnoPlacanje := ukupanIznos

	nacinPlacanja := nalog.NacinPlacanja
	if nacinPlacanja == "" {
		nacinPlacanja = "Gotovina"
	}

	zahtev := InvoiceRequest{
		InvoiceRequest: InvoiceRequestBody{
			InvoiceType:     "Normal",
			TransactionType: "Sale",
			Payment: []PaymentItem{
				{
					Amount:      ukupnoPlacanje,
					PaymentType: TipPlacanja(nacinPlacanja),
				},
			},
			Items:   items,
			Cashier: kasir,
		},
	}

	// buyerId: "10:PIB" za pravno lice, "01:JMBG" za fizičko
	if pib != "" {
		zahtev.InvoiceRequest.BuyerID = "10:" + pib
	} else if jmbg != "" {
		zahtev.InvoiceRequest.BuyerID = "01:" + jmbg
	}

	return zahtev
}

// NapraviRefundZahtev gradi Refund zahtev za storno fiskalnog računa.
// referentBroj je PfrBroj originalnog fiskalnog računa (invoiceNumber iz PFR odgovora).
func NapraviRefundZahtev(
	nalog model.ProdajniNalog,
	stavke []model.StavkaProdajeSaArtiklom,
	pib, jmbg, kasir, referentBroj string,
) InvoiceRequest {
	zahtev := NapraviZahtev(nalog, stavke, pib, jmbg, kasir)
	zahtev.InvoiceRequest.TransactionType = "Refund"
	zahtev.InvoiceRequest.ReferentDocumentNumber = referentBroj
	return zahtev
}
