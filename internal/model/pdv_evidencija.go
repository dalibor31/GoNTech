package model

import (
	"fmt"
	"math"
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

// dodajStavku razvrstava jednu stavku (već izračunata osnovica + PDV) po stopi u
// odgovarajuće kolone KIR-a i uvećava ukupan (bruto) iznos.
func (k *PdvKir) dodajStavku(osnovica, pdv, stopa float64) {
	switch stopa {
	case 20:
		k.OsnovicaOpsta += osnovica
		k.PdvOpsta += pdv
	case 10:
		k.OsnovicaPosebna += osnovica
		k.PdvPosebna += pdv
	default:
		// 0% / oslobođeno — osnovica u oslobođen promet sa pravom na odbitak
		k.OslobodenSaPravom += osnovica
	}
	k.Ukupno += osnovica + pdv
}

// DodajNeto dodaje stavku zadatu NETO osnovicom i PDV stopom — za izvore gde je
// cena bez PDV-a (servisni rad/deo: cena_komada je neto, PDV se dodaje naviše).
// PDV se izvodi kao osnovica × stopa/100, a ukupno postaje bruto (osnovica + PDV).
func (k *PdvKir) DodajNeto(osnovica, stopa float64) {
	k.dodajStavku(osnovica, osnovica*stopa/100, stopa)
}

// KirIzProdaje gradi KIR zapis iz prodaje: stavke se grupišu po PDV stopi
// (20→opšta, 10→posebna, ostalo→oslobođeno). Koristi već izračunatu CenaBezPdv
// (neto, popust je već uračunat pri upisu stavke — v. ProdajaRepo.Kreiraj), jer je
// CenaPoKomadu kod punog PDV obveznika bruto (cena za naplatu), ne neto.
// DatumPrometa je datum same prodaje (nalog.Datum); DatumKnjizenja je datum kada se
// ovaj KIR zapis stvarno upisuje u knjigu — za prodaju upisanu odmah to je isti trenutak,
// ali za naknadni backfill starih naloga upis kasni za periodom pa se prosleđuje posebno.
func KirIzProdaje(nalog ProdajniNalog, stavke []StavkaProdaje, kupacNaziv, kupacPib, kupacMesto string, datumKnjizenja time.Time) PdvKir {
	id := nalog.ID
	k := PdvKir{
		DatumPrometa:   nalog.Datum,
		DatumKnjizenja: datumKnjizenja,
		BrojDokumenta:  nalog.BrojNaloga,
		KupacNaziv:     kupacNaziv,
		KupacPib:       kupacPib,
		KupacMesto:     kupacMesto,
		Izvor:          "prodaja",
		IzvorID:        &id,
	}
	for _, s := range stavke {
		k.DodajNeto(float64(s.Kolicina)*s.CenaBezPdv, s.PdvStopa)
	}
	return k
}

// KirIzServisa gradi KIR zapis iz preuzetog servisnog naloga: radovi i ugrađeni
// delovi (bez predloženih koji čekaju odobrenje klijenta) se sabiraju kao NETO
// stavke po svojoj PDV stopi. datumPrometa je trenutak prometa (za sveže naloge
// „sada", za backfill starih naloga datum preuzimanja); datumKnjizenja je kad se
// zapis stvarno upisuje u knjigu (za backfill kasni za periodom).
func KirIzServisa(nalog ServisniNalog, radovi []ServisniRad, delovi []ServisniDeoSaArtiklom, kupacNaziv, kupacPib, kupacMesto string, datumPrometa, datumKnjizenja time.Time) PdvKir {
	id := nalog.ID
	k := PdvKir{
		DatumPrometa:   datumPrometa,
		DatumKnjizenja: datumKnjizenja,
		BrojDokumenta:  nalog.BrojNaloga,
		KupacNaziv:     kupacNaziv,
		KupacPib:       kupacPib,
		KupacMesto:     kupacMesto,
		Izvor:          "servis",
		IzvorID:        &id,
	}
	for _, r := range radovi {
		if r.Predlozeno {
			continue
		}
		k.DodajNeto(r.Ukupno(), r.PdvStopa)
	}
	for _, d := range delovi {
		if d.Predlozeno {
			continue
		}
		k.DodajNeto(d.Ukupno(), d.PdvStopa)
	}
	return k
}

// KirStorno gradi storno (knjižno odobrenje) zapis za dati KIR red: iznosi su
// negirani original, upisan pod istim izvorom/izvor_id tako da PostojiZaIzvor
// ostane tačan i backfill ne pokuša ponovni upis. Zakon o računovodstvu ne
// dozvoljava brisanje knjigovodstvenih evidencija — storno se knjiži kao
// poništavajuća stavka koja upućuje na original, a ne kao brisanje reda.
func KirStorno(original PdvKir, razlog string, datumStorna time.Time) PdvKir {
	napomena := "Storno računa " + original.BrojDokumenta
	if razlog != "" {
		napomena += ": " + razlog
	}
	return PdvKir{
		DatumPrometa:      datumStorna,
		DatumKnjizenja:    datumStorna,
		BrojDokumenta:     "STORNO-" + original.BrojDokumenta,
		KupacNaziv:        original.KupacNaziv,
		KupacPib:          original.KupacPib,
		KupacMesto:        original.KupacMesto,
		OsnovicaOpsta:     -original.OsnovicaOpsta,
		PdvOpsta:          -original.PdvOpsta,
		OsnovicaPosebna:   -original.OsnovicaPosebna,
		PdvPosebna:        -original.PdvPosebna,
		OslobodenSaPravom: -original.OslobodenSaPravom,
		OslobodenBezPrava: -original.OslobodenBezPrava,
		Ukupno:            -original.Ukupno,
		Napomena:          napomena,
		Izvor:             original.Izvor,
		IzvorID:           original.IzvorID,
	}
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
	Uvoz             bool   // true = uvoz (JCI) → PPPDV 006/106; false = domaća nabavka → 008/108
	DatumUnosa       time.Time
}

// KprStorno gradi storno zapis za dati KPR red — vidi KirStorno.
func KprStorno(original PdvKpr, razlog string, datumStorna time.Time) PdvKpr {
	napomena := "Storno dokumenta " + original.BrojDokumenta
	if razlog != "" {
		napomena += ": " + razlog
	}
	return PdvKpr{
		DatumPrometa:     datumStorna,
		DatumKnjizenja:   datumStorna,
		BrojDokumenta:    "STORNO-" + original.BrojDokumenta,
		DobavljacNaziv:   original.DobavljacNaziv,
		DobavljacPib:     original.DobavljacPib,
		DobavljacMesto:   original.DobavljacMesto,
		OsnovicaOpsta:    -original.OsnovicaOpsta,
		PdvOpsta:         -original.PdvOpsta,
		OsnovicaPosebna:  -original.OsnovicaPosebna,
		PdvPosebna:       -original.PdvPosebna,
		PdvBezOdbitka:    -original.PdvBezOdbitka,
		OslobodenNabavka: -original.OslobodenNabavka,
		Ukupno:           -original.Ukupno,
		Napomena:         napomena,
		Izvor:            original.Izvor,
		IzvorID:          original.IzvorID,
		Uvoz:             original.Uvoz,
	}
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

// KprIzNabavke gradi KPR zapis iz nabavke.
// Ako nabavka ima BrojRacuna, koristi ga kao BrojDokumenta; inače sintetički "NAB-<id>".
// Ako nabavka ima DatumRacuna, koristi ga kao DatumPrometa; inače datum nabavke.
// Ako nabavka ima PdvIznos > 0, koristi ga direktno (stvarni PDV sa računa dobavljača)
// raspoređen proporcionalno po stavkama; inače aproksimacija iz stope artikla.
func KprIzNabavke(nabavka Nabavka, dobavljacNaziv, dobavljacPib, dobavljacMesto string, stavke []NabavkaStavkaPdv) PdvKpr {
	id := nabavka.ID

	brojDok := fmt.Sprintf("NAB-%d", id)
	if nabavka.BrojRacuna != "" {
		brojDok = nabavka.BrojRacuna
	}

	datumPrometa := nabavka.Datum
	if nabavka.DatumRacuna != nil {
		datumPrometa = *nabavka.DatumRacuna
	}

	k := PdvKpr{
		DatumPrometa:   datumPrometa,
		DatumKnjizenja: nabavka.Datum,
		DatumPlacanja:  nabavka.DatumPlacanja,
		BrojDokumenta:  brojDok,
		DobavljacNaziv: dobavljacNaziv,
		DobavljacPib:   dobavljacPib,
		DobavljacMesto: dobavljacMesto,
		Napomena:       nabavka.Napomena,
		Izvor:          "nabavka",
		IzvorID:        &id,
	}

	// ako je unesen stvarni PDV sa računa, raspodeli ga proporcionalno po stavkama
	if nabavka.PdvIznos > 0 {
		var ukupnaOsnovica float64
		for _, s := range stavke {
			ukupnaOsnovica += s.Osnovica
		}
		for _, s := range stavke {
			var udeo float64
			if ukupnaOsnovica > 0 {
				udeo = s.Osnovica / ukupnaOsnovica
			}
			pdv := math.Round(nabavka.PdvIznos*udeo*100) / 100
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
	} else {
		// aproksimacija: PDV iz stope artikla
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
	}
	return k
}

// PdvObracun je rezultat obračuna PDV za period: izlazni (dugovani) PDV iz KIR,
// prethodni (odbitni) PDV iz KPR i konačna obaveza. Pozitivna obaveza znači iznos
// za uplatu, negativna iznos za povraćaj / prenos poreskog kredita u naredni period.
type PdvObracun struct {
	// izlazni (dugovani) PDV — iz KIR, po stopama
	IzlazniPdvOpsta   float64
	IzlazniPdvPosebna float64
	IzlazniPdvUkupno  float64
	// prethodni (odbitni) PDV — iz KPR, po stopama (bez PDV bez prava na odbitak)
	OdbitniPdvOpsta   float64
	OdbitniPdvPosebna float64
	OdbitniPdvUkupno  float64
	// obaveza = izlazni − odbitni (>0 za uplatu, <0 za povraćaj/prenos)
	Obaveza float64
}

// ZaUplatu vraća true kada postoji obaveza za uplatu (izlazni PDV veći od odbitnog).
func (o PdvObracun) ZaUplatu() bool {
	return o.Obaveza > 0
}

// ObavezaApsolutna vraća iznos obaveze bez predznaka (za prikaz povraćaja kao pozitivan broj).
func (o PdvObracun) ObavezaApsolutna() float64 {
	if o.Obaveza < 0 {
		return -o.Obaveza
	}
	return o.Obaveza
}

// PPPDV su vrednosti polja zvaničnog obrasca poreske prijave PDV (PPPDV).
// Iznosi su u celim dinarima (obrazac ne dozvoljava decimale). Brojevi polja
// odgovaraju obrascu: 001–005/103–105 (promet i izlazni PDV), 006–009/106–109
// (prethodni/odbitni porez), 110 (obaveza = 105 − 109), Povracaj (polje 11).
type PPPDV struct {
	// I. promet dobara i usluga
	Polje001 int64 // oslobođen sa pravom na odbitak (naknada)
	Polje002 int64 // oslobođen bez prava na odbitak (naknada)
	Polje003 int64 // opšta stopa — naknada bez PDV
	Polje103 int64 // opšta stopa — obračunati PDV
	Polje004 int64 // posebna stopa — naknada bez PDV
	Polje104 int64 // posebna stopa — obračunati PDV
	Polje005 int64 // zbir naknada (1+2+3+4)
	Polje105 int64 // zbir izlaznog PDV (103+104)
	// II. prethodni porez
	Polje006 int64 // prethodni porez pri uvozu — naknada (ne prati se → 0)
	Polje106 int64 // prethodni porez pri uvozu — PDV (ne prati se → 0)
	Polje007 int64 // PDV nadoknada poljoprivredniku — naknada (ne prati se → 0)
	Polje107 int64 // PDV nadoknada poljoprivredniku — PDV (ne prati se → 0)
	Polje008 int64 // ostali prethodni porez — naknada
	Polje108 int64 // ostali prethodni porez — PDV (odbitni)
	Polje009 int64 // zbir naknada (6+7+8)
	Polje109 int64 // zbir prethodnog poreza (106+107+108)
	// III. poreska obaveza
	Polje110 int64 // iznos PDV u periodu (105 − 109); >0 za uplatu, <0 za povraćaj
	Povracaj bool  // polje 11: true kada je 110 negativan (iznos za povraćaj)
}

// Polje110Apsolutno vraća iznos polja 110 bez predznaka (za prikaz povraćaja).
func (p PPPDV) Polje110Apsolutno() int64 {
	if p.Polje110 < 0 {
		return -p.Polje110
	}
	return p.Polje110
}

// MapirajPPPDV preslikava zbirove KIR i KPR na polja obrasca PPPDV (cele dinare).
// KPR je razdvojen: uvoz (JCI) ide u polja 006/106 (prethodni porez pri uvozu),
// domaća nabavka u 008/108 (ostali prethodni porez).
// ⚠ PDV nadoknada poljoprivredniku (007/107) se i dalje ne prati zasebno (ostaje 0).
func MapirajPPPDV(kir PdvKirSume, kprDomace, kprUvoz PdvKprSume) PPPDV {
	uDinare := func(v float64) int64 { return int64(math.Round(v)) }
	p := PPPDV{
		Polje001: uDinare(kir.OslobodenSaPravom),
		Polje002: uDinare(kir.OslobodenBezPrava),
		Polje003: uDinare(kir.OsnovicaOpsta),
		Polje103: uDinare(kir.PdvOpsta),
		Polje004: uDinare(kir.OsnovicaPosebna),
		Polje104: uDinare(kir.PdvPosebna),
		Polje006: uDinare(kprUvoz.OsnovicaOpsta + kprUvoz.OsnovicaPosebna),
		Polje106: uDinare(kprUvoz.PdvOpsta + kprUvoz.PdvPosebna),
		Polje008: uDinare(kprDomace.OsnovicaOpsta + kprDomace.OsnovicaPosebna),
		Polje108: uDinare(kprDomace.PdvOpsta + kprDomace.PdvPosebna),
	}
	// zbirovi se računaju iz zaokruženih polja da kolone na obrascu uvek zbiraju
	p.Polje005 = p.Polje001 + p.Polje002 + p.Polje003 + p.Polje004
	p.Polje105 = p.Polje103 + p.Polje104
	p.Polje009 = p.Polje006 + p.Polje007 + p.Polje008
	p.Polje109 = p.Polje106 + p.Polje107 + p.Polje108
	p.Polje110 = p.Polje105 - p.Polje109
	p.Povracaj = p.Polje110 < 0
	return p
}

// ObracunajPdv računa obavezu PDV iz zbirova KIR i KPR za isti period.
// PdvBezOdbitka iz KPR se namerno NE računa u odbitni PDV — to je PDV za koji
// ne postoji pravo na odbitak prethodnog poreza.
func ObracunajPdv(kir PdvKirSume, kpr PdvKprSume) PdvObracun {
	o := PdvObracun{
		IzlazniPdvOpsta:   kir.PdvOpsta,
		IzlazniPdvPosebna: kir.PdvPosebna,
		OdbitniPdvOpsta:   kpr.PdvOpsta,
		OdbitniPdvPosebna: kpr.PdvPosebna,
	}
	o.IzlazniPdvUkupno = o.IzlazniPdvOpsta + o.IzlazniPdvPosebna
	o.OdbitniPdvUkupno = o.OdbitniPdvOpsta + o.OdbitniPdvPosebna
	o.Obaveza = o.IzlazniPdvUkupno - o.OdbitniPdvUkupno
	return o
}
