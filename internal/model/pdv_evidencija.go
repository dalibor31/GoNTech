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
