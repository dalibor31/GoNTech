package auth

import "testing"

func TestGenerisiRezervneKodove(t *testing.T) {
	kodovi, err := GenerisiRezervneKodove(10)
	if err != nil {
		t.Fatalf("GenerisiRezervneKodove: %v", err)
	}
	if len(kodovi) != 10 {
		t.Fatalf("dobijeno %d kodova, očekivano 10", len(kodovi))
	}
	vidjeni := map[string]bool{}
	for _, k := range kodovi {
		if len(k) != 9 || k[4] != '-' {
			t.Fatalf("kod nije u formatu XXXX-XXXX: %q", k)
		}
		if vidjeni[k] {
			t.Fatalf("ponovljen kod: %q", k)
		}
		vidjeni[k] = true
	}
}

func TestNormalizujRezervniKod(t *testing.T) {
	slucajevi := map[string]string{
		"ABCD-2345":         "ABCD-2345",
		"abcd2345":          "ABCD-2345",
		"abcd 2345":         "ABCD-2345",
		" a b c d 2 3 4 5 ": "ABCD-2345",
	}
	for ulaz, ocek := range slucajevi {
		if got := NormalizujRezervniKod(ulaz); got != ocek {
			t.Errorf("NormalizujRezervniKod(%q) = %q, očekivano %q", ulaz, got, ocek)
		}
	}
}
