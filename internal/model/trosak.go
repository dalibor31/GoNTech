package model

// Trosak je stavka šifrarnika vrsta troškova (npr. Prevoz, Carina, Ambalaža).
// To je katalog dodatnih troškova koji se vezuju za radni nalog i ulaze u cenu
// koštanja. NIJE evidencija rashoda firme (struja, kirija) — to je poseban modul.
type Trosak struct {
	ID         int64
	Sifra      string
	Naziv      string
	Cena       float64
	Opis       string
	Arhiviran  bool
	DatumUnosa string
}
