package model

import "time"

// Korisnik predstavlja nalog korisnika u sistemu
type Korisnik struct {
	ID             int64
	KorisnickoIme  string
	LozinkaHash    string
	Uloga          string // "superadmin" | "admin" | "radnik"
	Aktivan        bool
	TotpTajna         string
	LokalnaTema       string // "tamna" | "svetla" | ""
	KoristiLokalnuTemu bool
	DatumKreiranja    time.Time
	LokalnaPozadina              string
	LokalnaPozadinaOpacity       string
	LokalnaPozadinaBlur          string
	LokalnaPozadinaBlurPozadine  string
	LokalnaPozadinaGlassOpacity  string
	AvatarPutanja                string
	LokalnaAnimacija             string // "" | "fadeInUp" | "fadeIn" | "scaleIn" | "slideLeft"
	LokalniHover                 string // "" | "bez" | "podizanje" | "svetlost"
	LokalnaBrzinaAnimacije       string // "" | "0.2" | "0.4" | "0.6" | "0.8" | "1.2" (sekunde)
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
