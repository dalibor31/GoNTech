package db

// ErrNedovoljnoKolicine se vraća kada prodaja traži više nego što ima na stanju
type ErrNedovoljnoKolicine struct {
	ArtikalNaziv string
}

func (e *ErrNedovoljnoKolicine) Error() string {
	return "ntech: nedovoljno količine na stanju za artikal: " + e.ArtikalNaziv
}
