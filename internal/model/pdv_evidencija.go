package model

import "time"

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
