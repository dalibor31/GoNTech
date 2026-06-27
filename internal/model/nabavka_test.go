package model

import "testing"

// proverava raspodelu zavisnih troškova na stavke nabavke po oba metoda
func TestRasporediTroskove(t *testing.T) {
	// dve stavke: A = 10×100 (vrednost 1000), B = 10×200 (vrednost 2000)
	stavke := []StavkaNabavke{
		{Kolicina: 10, CenaPoKomadu: 100},
		{Kolicina: 10, CenaPoKomadu: 200},
	}

	slucajevi := []struct {
		naziv   string
		stavke  []StavkaNabavke
		trosak  float64
		metod   string
		ocekuje []float64
	}{
		{
			naziv:   "bez troška — cena nepromenjena",
			stavke:  stavke,
			trosak:  0,
			metod:   "vrednost",
			ocekuje: []float64{100, 200},
		},
		{
			// trošak 300 po vrednosti: A nosi 1/3 (100 → 10/kom), B nosi 2/3 (200 → 20/kom)
			naziv:   "po vrednosti",
			stavke:  stavke,
			trosak:  300,
			metod:   "vrednost",
			ocekuje: []float64{110, 220},
		},
		{
			// trošak 300 po količini: svaka stavka 1/2 (150 → 15/kom)
			naziv:   "po količini",
			stavke:  stavke,
			trosak:  300,
			metod:   "kolicina",
			ocekuje: []float64{115, 215},
		},
		{
			// osnovica po vrednosti je 0 (sve cene 0) — nema raspodele
			naziv:   "nulta osnovica vrednosti",
			stavke:  []StavkaNabavke{{Kolicina: 5, CenaPoKomadu: 0}},
			trosak:  100,
			metod:   "vrednost",
			ocekuje: []float64{0},
		},
		{
			// stavka bez količine se preskače (bez deljenja nulom)
			naziv:   "količina nula",
			stavke:  []StavkaNabavke{{Kolicina: 0, CenaPoKomadu: 50}},
			trosak:  100,
			metod:   "kolicina",
			ocekuje: []float64{50},
		},
		{
			// negativan trošak (odobreni popust) — cena ostaje nepromenjena
			naziv:   "negativan trosak",
			stavke:  stavke,
			trosak:  -100,
			metod:   "vrednost",
			ocekuje: []float64{100, 200},
		},
		{
			// trošak 10 na 3 stavke po kolicini (9 kom ukupno) → 10/9/3 ≈ 1.111/kom
			naziv: "zaokruzivanje na 2 decimale",
			stavke: []StavkaNabavke{
				{Kolicina: 3, CenaPoKomadu: 100},
				{Kolicina: 3, CenaPoKomadu: 200},
				{Kolicina: 3, CenaPoKomadu: 50},
			},
			trosak:  10,
			metod:   "kolicina",
			ocekuje: []float64{101.11, 201.11, 51.11},
		},
	}

	for _, s := range slucajevi {
		t.Run(s.naziv, func(t *testing.T) {
			dobijeno := RasporediTroskove(s.stavke, s.trosak, s.metod)
			if len(dobijeno) != len(s.ocekuje) {
				t.Fatalf("dužina = %d, očekivano %d", len(dobijeno), len(s.ocekuje))
			}
			for i := range s.ocekuje {
				if dobijeno[i] != s.ocekuje[i] {
					t.Errorf("stavka %d: dobijeno %.2f, očekivano %.2f", i, dobijeno[i], s.ocekuje[i])
				}
			}
		})
	}
}
