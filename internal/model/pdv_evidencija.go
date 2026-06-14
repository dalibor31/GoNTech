package model

import (
	"fmt"
	"time"
)

// PdvKir je jedan zapis u knjizi izdatih računa (izlazni PDV).
// Iznosi se vode po vrsti stope (opšta/posebna) — vidi migraciju 041.
type PdvKir struct {
	ID                int64
	DatumPrometa      time.Time
	DatumKnjizenja    time.Time
	BrojDokumenta     string
	KupacNaziv        string
	KupacPib          string
	KupacMesto        string
	OsnovicaOpsta     float64
	PdvOpsta          float64
	OsnovicaPosebna   float64
	PdvPosebna        float64
	OslobodenSaPravom float64
	OslobodenBezPrava float64
	Ukupno            float64
	Napomena          string
	Izvor             string // "rucno" | "prodaja" | "nabavka"
	IzvorID           *int64 // id izvornog naloga (nil za ručni unos)
	DatumUnosa        time.Time
}

// OslobodenUkupno vraća zbir oslobođenog prometa (sa i bez prava na odbitak).
func (k PdvKir) OslobodenUkupno() float64 {
	return k.OslobodenSaPravom + k.OslobodenBezPrava
}

// OznakaPoreskogBroja vraća „JMBG" ako uneti broj ima 13 cifara (fizičko lice),
// inače „PIB" (pravno lice / preduzetnik — PIB ima 9 cifara).
func (k PdvKir) OznakaPoreskogBroja() string {
	cifre := 0
	for _, r := range k.KupacPib {
		if r >= '0' && r <= '9' {
			cifre++
		}
	}
	if cifre == 13 {
		return "JMBG"
	}
	return "PIB"
}

// PdvKirSume su zbirovi kolona KIR-a (za red „ukupno" u pregledu knjige).
type PdvKirSume struct {
	OsnovicaOpsta     float64
	PdvOpsta          float64
	OsnovicaPosebna   float64
	PdvPosebna        float64
	OslobodenSaPravom float64
	OslobodenBezPrava float64
	Ukupno            float64
}

// OslobodenUkupno vraća zbir oslobođenog prometa (sa i bez prava na odbitak).
func (s PdvKirSume) OslobodenUkupno() float64 {
	return s.OslobodenSaPravom + s.OslobodenBezPrava
}

// SumirajKir sabira sve kolone iz liste KIR zapisa.
func SumirajKir(zapisi []PdvKir) PdvKirSume {
	var s PdvKirSume
	for _, z := range zapisi {
		s.OsnovicaOpsta += z.OsnovicaOpsta
		s.PdvOpsta += z.PdvOpsta
		s.OsnovicaPosebna += z.OsnovicaPosebna
		s.PdvPosebna += z.PdvPosebna
		s.OslobodenSaPravom += z.OslobodenSaPravom
		s.OslobodenBezPrava += z.OslobodenBezPrava
		s.Ukupno += z.Ukupno
	}
	return s
}

// KirIzProdaje gradi KIR zapis iz prodaje: stavke se grupišu po PDV stopi
// (20→opšta, 10→posebna, ostalo→oslobođeno). CenaPoKomadu je prodajna cena SA PDV,
// pa se osnovica izvodi deljenjem sa (1 + stopa/100).
func KirIzProdaje(nalog ProdajniNalog, stavke []StavkaProdaje, kupacNaziv, kupacPib, kupacMesto string) PdvKir {
	id := nalog.ID
	k := PdvKir{
		DatumPrometa:   nalog.Datum,
		DatumKnjizenja: nalog.Datum,
		BrojDokumenta:  nalog.BrojNaloga,
		KupacNaziv:     kupacNaziv,
		KupacPib:       kupacPib,
		KupacMesto:     kupacMesto,
		Izvor:          "prodaja",
		IzvorID:        &id,
	}
	for _, s := range stavke {
		ukupnoLinija := float64(s.Kolicina) * s.CenaPoKomadu
		osnovica := ukupnoLinija
		if s.PdvStopa > 0 {
			osnovica = ukupnoLinija / (1 + s.PdvStopa/100)
		}
		pdv := ukupnoLinija - osnovica
		switch s.PdvStopa {
		case 20:
			k.OsnovicaOpsta += osnovica
			k.PdvOpsta += pdv
		case 10:
			k.OsnovicaPosebna += osnovica
			k.PdvPosebna += pdv
		default:
			// 0% / oslobođeno — osnovica bez PDV-a u oslobođen promet sa pravom na odbitak
			k.OslobodenSaPravom += osnovica
		}
		k.Ukupno += ukupnoLinija
	}
	return k
}

// PdvKpr je jedan zapis u knjizi primljenih računa (ulazni PDV).
type PdvKpr struct {
	ID               int64
	DatumPrometa     time.Time
	DatumKnjizenja   time.Time
	DatumPlacanja    *time.Time // može biti prazan
	BrojDokumenta    string
	DobavljacNaziv   string
	DobavljacPib     string
	DobavljacMesto   string
	OsnovicaOpsta    float64
	PdvOpsta         float64
	OsnovicaPosebna  float64
	PdvPosebna       float64
	PdvBezOdbitka    float64
	OslobodenNabavka float64
	Ukupno           float64
	Napomena         string
	Izvor            string // "rucno" | "prodaja" | "nabavka"
	IzvorID          *int64 // id izvorne nabavke (nil za ručni unos)
	DatumUnosa       time.Time
}

// OznakaPoreskogBroja vraća „JMBG" za 13-cifreni broj, inače „PIB" (dobavljači su obično firme).
func (k PdvKpr) OznakaPoreskogBroja() string {
	cifre := 0
	for _, r := range k.DobavljacPib {
		if r >= '0' && r <= '9' {
			cifre++
		}
	}
	if cifre == 13 {
		return "JMBG"
	}
	return "PIB"
}

// PdvKprSume su zbirovi kolona KPR-a (za red „ukupno" u pregledu knjige).
type PdvKprSume struct {
	OsnovicaOpsta    float64
	PdvOpsta         float64
	OsnovicaPosebna  float64
	PdvPosebna       float64
	PdvBezOdbitka    float64
	OslobodenNabavka float64
	Ukupno           float64
}

// SumirajKpr sabira sve kolone iz liste KPR zapisa.
func SumirajKpr(zapisi []PdvKpr) PdvKprSume {
	var s PdvKprSume
	for _, z := range zapisi {
		s.OsnovicaOpsta += z.OsnovicaOpsta
		s.PdvOpsta += z.PdvOpsta
		s.OsnovicaPosebna += z.OsnovicaPosebna
		s.PdvPosebna += z.PdvPosebna
		s.PdvBezOdbitka += z.PdvBezOdbitka
		s.OslobodenNabavka += z.OslobodenNabavka
		s.Ukupno += z.Ukupno
	}
	return s
}

// NabavkaStavkaPdv je jedna stavka nabavke sa pripadajućom PDV stopom (iz artikla).
type NabavkaStavkaPdv struct {
	Osnovica float64 // nabavna vrednost stavke (cena × količina) — tretira se kao osnovica bez PDV
	PdvStopa float64
}

// KprIzNabavke gradi KPR zapis iz nabavke. PDV se izvodi iz stope artikla po stavci
// (⚠ aproksimacija: nabavna cena = osnovica bez PDV; stvaran PDV sa računa dobavljača može
// se razlikovati). Grupiše po stopi (20→opšta, 10→posebna, ostalo→oslobođena nabavka).
// Broj dokumenta je sintetički ("NAB-<id>") jer nabavka ne čuva broj računa dobavljača.
func KprIzNabavke(nabavka Nabavka, dobavljacNaziv, dobavljacPib, dobavljacMesto string, stavke []NabavkaStavkaPdv) PdvKpr {
	id := nabavka.ID
	k := PdvKpr{
		DatumPrometa:   nabavka.Datum,
		DatumKnjizenja: nabavka.Datum,
		BrojDokumenta:  fmt.Sprintf("NAB-%d", id),
		DobavljacNaziv: dobavljacNaziv,
		DobavljacPib:   dobavljacPib,
		DobavljacMesto: dobavljacMesto,
		Napomena:       nabavka.Napomena,
		Izvor:          "nabavka",
		IzvorID:        &id,
	}
	for _, s := range stavke {
		pdv := s.Osnovica * s.PdvStopa / 100
		switch s.PdvStopa {
		case 20:
			k.OsnovicaOpsta += s.Osnovica
			k.PdvOpsta += pdv
		case 10:
			k.OsnovicaPosebna += s.Osnovica
			k.PdvPosebna += pdv
		default:
			k.OslobodenNabavka += s.Osnovica
		}
		k.Ukupno += s.Osnovica + pdv
	}
	return k
}
