package model

import "time"

// PdvStopa je jedna stavka u šifarniku PDV stopa (Faza 1 knjigovodstvenog modula).
// Stope su podatak, ne hardkod — nova ili izmenjena stopa bez diranja koda.
type PdvStopa struct {
	ID         int64
	Naziv      string  // npr. "Opšta stopa"
	Stopa      float64 // procenat, npr. 20.0
	Oznaka     string  // "opsta" | "posebna" | "oslobodjeno"
	Aktivna    bool    // false = arhivirana (ne nudi se u listama, ali stari zapisi ostaju ispravni)
	Redosled   int     // redosled prikaza
	DatumUnosa time.Time
}
