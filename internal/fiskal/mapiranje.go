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
// pib/jmbg se koriste za buyerId identifikaciju kupca. primljeno je iznos koji je
// kupac stvarno predao (za obračun povraćaja na fiskalnom računu) — ako je 0 ili
// manji od ukupnog iznosa, šalje se tačan iznos duga (bez povraćaja).
func NapraviZahtev(
	nalog model.ProdajniNalog,
	stavke []model.StavkaProdajeSaArtiklom,
	pib, jmbg, kasir string,
	primljeno float64,
) InvoiceRequest {
	items := make([]InvoiceItem, 0, len(stavke))
	ukupanIznos := 0.0

	for _, s := range stavke {
		// stopa je uvek ono što je snimljeno na stavci — 0 je legitimna vrednost
		// (van sistema PDV/neobveznik), ne „nije postavljeno". Nikad je ne prepisivati
		// nazad na opštu stopu, jer bi to poništilo nuliranje PDV-a za ne-PDV obveznike
		// (v. docs/Greške.md).
		stopa := s.PdvStopa
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

	// ako je kupac dao više od duga, na računu se iskazuje stvarno primljen iznos
	// (fiskalni uređaj sam izračunava povraćaj kao razliku)
	iznosPlacanja := ukupnoPlacanje
	if primljeno > ukupnoPlacanje {
		iznosPlacanja = primljeno
	}

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
					Amount:      iznosPlacanja,
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
	zahtev := NapraviZahtev(nalog, stavke, pib, jmbg, kasir, 0)
	zahtev.InvoiceRequest.TransactionType = "Refund"
	zahtev.InvoiceRequest.ReferentDocumentNumber = referentBroj
	return zahtev
}

// NazivAvansneStavke je naziv stavke na avansnim fiskalnim računima (isti naziv
// mora biti dosledan, jer ga PFR ne povezuje po ID-u već po sadržaju stavke).
const NazivAvansneStavke = "Аванс"

// NapraviAvansZahtev gradi Advance/Sale zahtev za primljeni avans — jedna stavka
// "Аванс" u punom (bruto) iznosu. stopa je PDV stopa avansa — u trenutku uplate
// stavke (radovi/delovi) obično još nisu poznate, pa pozivalac prosleđuje opštu
// stopu iz šifarnika (Podešavanja → Kalkulacija i PDV) kad je firma PDV obveznik,
// ili 0 kad nije — nikad hardkodovanu vrednost (v. docs/Greške.md §4.1).
func NapraviAvansZahtev(iznos, stopa float64, nacinPlacanja, kasir string) InvoiceRequest {
	return InvoiceRequest{
		InvoiceRequest: InvoiceRequestBody{
			InvoiceType:     "Advance",
			TransactionType: "Sale",
			Payment: []PaymentItem{
				{Amount: iznos, PaymentType: TipPlacanja(nacinPlacanja)},
			},
			Items: []InvoiceItem{
				{Name: NazivAvansneStavke, Labels: []string{OznakaPDV(stopa)}, TotalAmount: iznos, UnitPrice: iznos, Quantity: 1},
			},
			Cashier: kasir,
		},
	}
}

// NapraviAvansRefundZahtev gradi Advance/Refund zahtev za povraćaj dela ili celog
// avansa (npr. kad avans premaši konačnu cenu popravke). referentBroj je PfrBroj
// originalnog avansnog računa.
func NapraviAvansRefundZahtev(iznos, stopa float64, nacinPlacanja, kasir, referentBroj string) InvoiceRequest {
	zahtev := NapraviAvansZahtev(iznos, stopa, nacinPlacanja, kasir)
	zahtev.InvoiceRequest.TransactionType = "Refund"
	zahtev.InvoiceRequest.ReferentDocumentNumber = referentBroj
	return zahtev
}

// PorezIzBrutoAvansa izvlači poreski deo iz bruto iznosa avansa po datoj stopi
// (npr. 500 din avansa po 20% → porez ≈ 83.33 din). stopa=0 (ne-PDV obveznik) → 0.
func PorezIzBrutoAvansa(bruto, stopa float64) float64 {
	if stopa <= 0 {
		return 0
	}
	return math.Round(bruto*stopa/(100+stopa)*100) / 100
}
