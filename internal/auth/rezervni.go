package auth

import (
	"crypto/rand"
	"fmt"
	"strings"
)

// alfabetKodova je Crockford base32 (32 znaka — deli 256 bez ostatka, pa nema
// modulo pristrasnosti) — izostavlja dvosmislene znakove I, L, O, U.
const alfabetKodova = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// BrojRezervnihKodova je podrazumevani broj kodova koji se generiše pri aktivaciji.
const BrojRezervnihKodova = 10

// GenerisiRezervneKodove vraća n nasumičnih jednokratnih kodova formata "XXXX-XXXX".
// Vraća čist tekst — pozivalac ih prikazuje korisniku jednom i čuva samo bcrypt heš.
func GenerisiRezervneKodove(n int) ([]string, error) {
	kodovi := make([]string, 0, n)
	for i := 0; i < n; i++ {
		kod, err := nasumicanKod()
		if err != nil {
			return nil, fmt.Errorf("ntech: auth.GenerisiRezervneKodove: %w", err)
		}
		kodovi = append(kodovi, kod)
	}
	return kodovi, nil
}

// nasumicanKod gradi jedan kod "XXXX-XXXX" iz alfabetKodova
func nasumicanKod() (string, error) {
	bajtovi := make([]byte, 8)
	if _, err := rand.Read(bajtovi); err != nil {
		return "", err
	}
	var sb strings.Builder
	for i, b := range bajtovi {
		if i == 4 {
			sb.WriteByte('-')
		}
		sb.WriteByte(alfabetKodova[int(b)%len(alfabetKodova)])
	}
	return sb.String(), nil
}

// NormalizujRezervniKod dovodi korisnikov unos u kanonski oblik "XXXX-XXXX":
// uklanja razmake/crtice, prebacuje u velika slova i ubacuje crticu na sredini.
// Tako se isti kod prepoznaje bez obzira na to kako ga korisnik otkuca.
func NormalizujRezervniKod(unos string) string {
	var sb strings.Builder
	for _, r := range strings.ToUpper(unos) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			sb.WriteRune(r)
		}
	}
	s := sb.String()
	if len(s) == 8 {
		return s[:4] + "-" + s[4:]
	}
	return s
}
