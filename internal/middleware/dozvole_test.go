package middleware

import "testing"

func TestImaDozvoluSuperadmin(t *testing.T) {
	// superadmin sme sve, uključujući akcije koje ni ne postoje
	for _, akcija := range []string{"artikal.obrisi", "prodaja.storno", "backup.pokreni", "nepostojeca.akcija"} {
		if !ImaDozvolu("superadmin", akcija) {
			t.Errorf("superadmin bi morao da sme %q", akcija)
		}
	}
}

func TestImaDozvoluAdmin(t *testing.T) {
	sme := []string{"artikal.obrisi", "prodaja.storno", "backup.pokreni", "izvestaj.pregled", "podesavanja.izmeni", "dashboard.prihod"}
	for _, a := range sme {
		if !ImaDozvolu("admin", a) {
			t.Errorf("admin bi morao da sme %q", a)
		}
	}
	// admin ne sme nepostojeću akciju
	if ImaDozvolu("admin", "nepostojeca.akcija") {
		t.Error("admin ne bi smeo nepostojeću akciju")
	}
}

func TestImaDozvoluRadnik(t *testing.T) {
	sme := []string{"kategorija.pregled", "dobavljac.dodaj", "servis.izmeni", "prodaja.dodaj", "klijent.izmeni", "tema.lokalno"}
	for _, a := range sme {
		if !ImaDozvolu("radnik", a) {
			t.Errorf("radnik bi morao da sme %q", a)
		}
	}
	// radnik NE sme: brisanje, storno, izveštaje, podešavanja, backup
	neSme := []string{"prodaja.storno", "servis.obrisi", "klijent.obrisi",
		"dobavljac.obrisi", "izvestaj.pregled", "podesavanja.izmeni", "backup.pokreni", "dashboard.prihod"}
	for _, a := range neSme {
		if ImaDozvolu("radnik", a) {
			t.Errorf("radnik NE bi smeo %q", a)
		}
	}
}

func TestImaDozvoluNepoznataUloga(t *testing.T) {
	if ImaDozvolu("gost", "kategorija.pregled") {
		t.Error("nepoznata uloga ne sme ništa")
	}
}

func TestSveDozvolePokrivaSveAkcije(t *testing.T) {
	m := SveDozvole("radnik")
	if len(m) != len(sveAkcije) {
		t.Fatalf("SveDozvole vraća %d akcija, očekivano %d", len(m), len(sveAkcije))
	}
	for _, a := range sveAkcije {
		if _, ok := m[a]; !ok {
			t.Errorf("akcija %q nedostaje u SveDozvole", a)
		}
	}
}
