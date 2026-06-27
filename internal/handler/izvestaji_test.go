package handler

import (
	"testing"
	"time"
)

// TestKljuceviMeseci proverava da se na kraju meseca (29–31.) nijedan mesec ne
// preskoči niti duplira — klasičan AddBug sa prelivom dana.
func TestKljuceviMeseci(t *testing.T) {
	slucajevi := []struct {
		naziv   string
		sada    time.Time
		n       int
		ocekuje []string
	}{
		{
			// 31. mart: bez sidrenja na 1. dan, "31. feb" bi se prelio u mart i
			// februar bi nestao. Mora dati uredan niz jan..mart.
			naziv:   "31. mart — bez preskakanja februara",
			sada:    time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC),
			n:       3,
			ocekuje: []string{"2026-01", "2026-02", "2026-03"},
		},
		{
			// 31. maj − unazad: mart/april imaju 31/30 dana
			naziv:   "31. maj — uredan niz unazad",
			sada:    time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC),
			n:       4,
			ocekuje: []string{"2026-02", "2026-03", "2026-04", "2026-05"},
		},
		{
			// prelaz preko godine
			naziv:   "30. januar — prelaz preko godine",
			sada:    time.Date(2026, 1, 30, 0, 0, 0, 0, time.UTC),
			n:       3,
			ocekuje: []string{"2025-11", "2025-12", "2026-01"},
		},
		{
			// puna godina (12 meseci), bez duplikata
			naziv: "12 meseci, bez duplikata",
			sada:  time.Date(2026, 6, 27, 0, 0, 0, 0, time.UTC),
			n:     12,
			ocekuje: []string{
				"2025-07", "2025-08", "2025-09", "2025-10", "2025-11", "2025-12",
				"2026-01", "2026-02", "2026-03", "2026-04", "2026-05", "2026-06",
			},
		},
	}

	for _, s := range slucajevi {
		t.Run(s.naziv, func(t *testing.T) {
			dobijeno := kljuceviMeseci(s.sada, s.n)
			if len(dobijeno) != len(s.ocekuje) {
				t.Fatalf("dužina = %d, očekivano %d (%v)", len(dobijeno), len(s.ocekuje), dobijeno)
			}
			vidjeni := map[string]bool{}
			for i := range s.ocekuje {
				if dobijeno[i] != s.ocekuje[i] {
					t.Errorf("[%d] = %q, očekivano %q", i, dobijeno[i], s.ocekuje[i])
				}
				if vidjeni[dobijeno[i]] {
					t.Errorf("mesec %q se duplira", dobijeno[i])
				}
				vidjeni[dobijeno[i]] = true
			}
		})
	}
}
