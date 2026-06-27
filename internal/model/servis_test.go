package model

import "testing"

func TestServisniNalog_PreostaloZaNaplatu(t *testing.T) {
	slucajevi := []struct {
		naziv     string
		konacna   *float64
		avans     *float64
		ocekivano *float64
	}{
		{
			naziv:     "cena_konacna nil → nil (nije uneta)",
			konacna:   nil,
			avans:     nil,
			ocekivano: nil,
		},
		{
			naziv:     "bez avansa → puna cena",
			konacna:   ptr(2000),
			avans:     nil,
			ocekivano: ptr(2000),
		},
		{
			naziv:     "avans manji od cene → razlika",
			konacna:   ptr(2000),
			avans:     ptr(500),
			ocekivano: ptr(1500),
		},
		{
			naziv:     "avans jednak ceni → 0",
			konacna:   ptr(2000),
			avans:     ptr(2000),
			ocekivano: ptr(0),
		},
		{
			naziv:     "avans veći od cene → 0 (ne ide u minus)",
			konacna:   ptr(1000),
			avans:     ptr(1500),
			ocekivano: ptr(0),
		},
	}

	for _, s := range slucajevi {
		t.Run(s.naziv, func(t *testing.T) {
			n := ServisniNalog{CenaKonacna: s.konacna, Avans: s.avans}
			dobijeno := n.PreostaloZaNaplatu()
			if s.ocekivano == nil {
				if dobijeno != nil {
					t.Errorf("dobijeno %v, očekivano nil", *dobijeno)
				}
				return
			}
			if dobijeno == nil {
				t.Fatalf("dobijeno nil, očekivano %.2f", *s.ocekivano)
			}
			if !jednako(*dobijeno, *s.ocekivano) {
				t.Errorf("dobijeno %.2f, očekivano %.2f", *dobijeno, *s.ocekivano)
			}
		})
	}
}

func TestServisniDeo_Ukupno(t *testing.T) {
	d := ServisniDeo{Kolicina: 3, CenaKomada: 150}
	if !jednako(d.Ukupno(), 450) {
		t.Errorf("Ukupno = %.2f, očekivano 450", d.Ukupno())
	}
}

func TestServisniDeoSaArtiklom_UkupnoSaPdv(t *testing.T) {
	d := ServisniDeoSaArtiklom{
		ServisniDeo: ServisniDeo{Kolicina: 2},
		CenaSaPdv:   600,
	}
	if !jednako(d.UkupnoSaPdv(), 1200) {
		t.Errorf("UkupnoSaPdv = %.2f, očekivano 1200", d.UkupnoSaPdv())
	}
}

func TestServisniRad_UkupnoIUkupnoSaPdv(t *testing.T) {
	r := ServisniRad{Kolicina: 2, CenaKomada: 1000, PdvStopa: 20, CenaSaPdv: 1200}
	if !jednako(r.Ukupno(), 2000) {
		t.Errorf("Ukupno = %.2f, očekivano 2000", r.Ukupno())
	}
	if !jednako(r.UkupnoSaPdv(), 2400) {
		t.Errorf("UkupnoSaPdv = %.2f, očekivano 2400", r.UkupnoSaPdv())
	}
}
