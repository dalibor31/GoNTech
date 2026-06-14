package model

import "testing"

func TestNivelacija(t *testing.T) {
	// poskupljenje: 100 → 120 = +20, +20%
	n := Nivelacija{StaraCena: 100, NovaCena: 120}
	if !blizu(n.Razlika(), 20) || !blizu(n.Procenat(), 20) || !n.Poskupljenje() {
		t.Errorf("razlika=%v procenat=%v poskupljenje=%v, očekivano 20/20/true", n.Razlika(), n.Procenat(), n.Poskupljenje())
	}

	// pojeftinjenje: 200 → 150 = -50, -25%
	n2 := Nivelacija{StaraCena: 200, NovaCena: 150}
	if !blizu(n2.Razlika(), -50) || !blizu(n2.Procenat(), -25) || n2.Poskupljenje() {
		t.Errorf("razlika=%v procenat=%v poskupljenje=%v, očekivano -50/-25/false", n2.Razlika(), n2.Procenat(), n2.Poskupljenje())
	}

	// stara cena 0 → procenat 0 (bez deljenja nulom)
	n3 := Nivelacija{StaraCena: 0, NovaCena: 80}
	if !blizu(n3.Procenat(), 0) {
		t.Errorf("procenat=%v, očekivano 0 kada je stara cena 0", n3.Procenat())
	}
}
