package model

import "time"

// Korisnik predstavlja nalog korisnika u sistemu
type Korisnik struct {
	ID             int64
	KorisnickoIme  string
	LozinkaHash    string
	Uloga          string // "superadmin" | "admin" | "radnik"
	Aktivan        bool
	TotpTajna      string
	DatumKreiranja time.Time
}

// Sesija predstavlja aktivnu sesiju prijavljenog korisnika
type Sesija struct {
	ID             int64
	KorisnikID     int64
	Token          string
	TotpPotvrdjeno bool
	DatumIsteka    time.Time
	DatumKreiranja time.Time
}
