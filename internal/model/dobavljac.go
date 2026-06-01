package model

import "time"

// Dobavljac predstavlja jednog dobavljača
type Dobavljac struct {
	ID           int64
	Naziv        string
	KontaktOsoba string
	Telefon      string
	Email        string
	Napomena     string
	DatumUnosa   time.Time
}
