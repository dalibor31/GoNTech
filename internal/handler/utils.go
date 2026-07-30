package handler

import (
	"fmt"
	"strconv"
	"strings"
)

// parseID parsira string ID iz URL parametra u int64
func parseID(s string) (int64, error) {
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("ntech: parseID: neispravan ID %q: %w", s, err)
	}
	return id, nil
}

// ignorisiKratkuPretragu vraća pretragu nepromenjenu ako je prazna ili ima 3+
// znaka, a inače "" (1-2 znaka se ignorišu). Ranije je ovo bio klijentski
// hx-trigger uslov (keyup[length==0||length>=3]) — uklonjen jer ga htmx
// evaluira preko eval()/new Function(), što CSP bez 'unsafe-eval' blokira
// Ovde se ista optimizacija radi na serveru, bez eval-a.
func ignorisiKratkuPretragu(pretraga string) string {
	if l := len([]rune(strings.TrimSpace(pretraga))); l > 0 && l < 3 {
		return ""
	}
	return pretraga
}
