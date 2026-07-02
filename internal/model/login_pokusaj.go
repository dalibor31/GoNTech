package model

import "time"

// LoginPokusaj predstavlja jedan zapis iz evidencije prijava
type LoginPokusaj struct {
	ID         int64
	KorisnikID *int64 // nil ako korisnik nije pronađen
	IP         string
	UserAgent  string
	Uspeh      bool
	Razlog     string
	Vreme      time.Time
}

// BlokiranaIP predstavlja IP adresu trenutno zaključanu zbog previše neuspelih pokušaja prijave
type BlokiranaIP struct {
	IP               string
	BrojNeuspeha     int
	PoslednjiPokusaj time.Time
}
