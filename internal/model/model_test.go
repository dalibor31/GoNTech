package model

import (
	"math"
	"testing"
)

// jednako proverava jednakost realnih brojeva uz malu toleranciju
func jednako(a, b float64) bool {
	return math.Abs(a-b) < 0.0001
}

func ptr(v float64) *float64 { return &v }

func TestArtikalCenaBezPdvIPdvIznos(t *testing.T) {
	testovi := []struct {
		naziv        string
		prodajna     float64 // neto
		cenaSaPdv    float64 // bruto
		stopa        float64
		ocekBezPdv   float64
		ocekPdvIznos float64
	}{
		{"stopa 20%", 100, 120, 20, 100, 20},
		{"stopa 0%", 100, 100, 0, 100, 0},
		{"stopa 10%", 100, 110, 10, 100, 10},
		{"nula cena", 0, 0, 20, 0, 0},
	}
	for _, tt := range testovi {
		t.Run(tt.naziv, func(t *testing.T) {
			a := Artikal{ProdajnaCena: tt.prodajna, CenaSaPdv: tt.cenaSaPdv, PdvStopa: tt.stopa}
			if got := a.CenaBezPdv(); !jednako(got, tt.ocekBezPdv) {
				t.Errorf("CenaBezPdv() = %v, očekivano %v", got, tt.ocekBezPdv)
			}
			if got := a.PdvIznos(); !jednako(got, tt.ocekPdvIznos) {
				t.Errorf("PdvIznos() = %v, očekivano %v", got, tt.ocekPdvIznos)
			}
		})
	}
}

func TestKlijentPunoIme(t *testing.T) {
	testovi := []struct {
		naziv string
		k     Klijent
		ocek  string
	}{
		{"pravno lice", Klijent{Tip: "pravno", NazivFirme: "Firma DOO", Ime: "x", Prezime: "y"}, "Firma DOO"},
		{"fizičko lice", Klijent{Tip: "fizicko", Ime: "Petar", Prezime: "Petrović"}, "Petar Petrović"},
		{"fizičko bez prezimena", Klijent{Tip: "fizicko", Ime: "Petar"}, "Petar"},
		{"fizičko prazno", Klijent{Tip: "fizicko"}, ""},
	}
	for _, tt := range testovi {
		t.Run(tt.naziv, func(t *testing.T) {
			if got := tt.k.PunoIme(); got != tt.ocek {
				t.Errorf("PunoIme() = %q, očekivano %q", got, tt.ocek)
			}
		})
	}
}

func TestServisniNalogPreostaloZaNaplatu(t *testing.T) {
	t.Run("bez konačne cene → nil", func(t *testing.T) {
		n := ServisniNalog{CenaKonacna: nil, Avans: ptr(50)}
		if got := n.PreostaloZaNaplatu(); got != nil {
			t.Errorf("očekivan nil, dobijeno %v", *got)
		}
	})
	t.Run("konačna minus avans", func(t *testing.T) {
		n := ServisniNalog{CenaKonacna: ptr(1000), Avans: ptr(300)}
		if got := n.PreostaloZaNaplatu(); got == nil || !jednako(*got, 700) {
			t.Errorf("očekivano 700, dobijeno %v", got)
		}
	})
	t.Run("bez avansa → puna cena", func(t *testing.T) {
		n := ServisniNalog{CenaKonacna: ptr(1000)}
		if got := n.PreostaloZaNaplatu(); got == nil || !jednako(*got, 1000) {
			t.Errorf("očekivano 1000, dobijeno %v", got)
		}
	})
	t.Run("avans veći od cene → 0 (ne negativno)", func(t *testing.T) {
		n := ServisniNalog{CenaKonacna: ptr(500), Avans: ptr(800)}
		if got := n.PreostaloZaNaplatu(); got == nil || !jednako(*got, 0) {
			t.Errorf("očekivano 0, dobijeno %v", got)
		}
	})
}
