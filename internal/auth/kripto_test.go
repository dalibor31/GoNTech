package auth

import (
	"crypto/rand"
	"testing"
)

// napraviKljuc vraća nasumičan 32-bajtni ključ za test
func napraviKljuc(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, DuzinaTotpKljuca)
	if _, err := rand.Read(k); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return k
}

// šifrovanje pa dešifrovanje vraća original
func TestSifrujDesifrujRoundTrip(t *testing.T) {
	kljuc := napraviKljuc(t)
	tajna := "JBSWY3DPEHPK3PXP"

	zapis, err := Sifruj(tajna, kljuc)
	if err != nil {
		t.Fatalf("Sifruj: %v", err)
	}
	if zapis == tajna {
		t.Fatal("šifrat je jednak originalu — nije šifrovano")
	}

	nazad, err := Desifruj(zapis, kljuc)
	if err != nil {
		t.Fatalf("Desifruj: %v", err)
	}
	if nazad != tajna {
		t.Fatalf("očekivano %q, dobijeno %q", tajna, nazad)
	}
}

// isti tekst dva puta daje različite šifrate (nasumičan nonce)
func TestSifrujNasumicanNonce(t *testing.T) {
	kljuc := napraviKljuc(t)
	a, _ := Sifruj("isti tekst", kljuc)
	b, _ := Sifruj("isti tekst", kljuc)
	if a == b {
		t.Fatal("dva šifrovanja istog teksta dala isti rezultat — nonce nije nasumičan")
	}
}

// dešifrovanje pogrešnim ključem mora da padne (integritet)
func TestDesifrujPogresanKljuc(t *testing.T) {
	zapis, _ := Sifruj("tajna", napraviKljuc(t))
	if _, err := Desifruj(zapis, napraviKljuc(t)); err == nil {
		t.Fatal("dešifrovanje pogrešnim ključem je uspelo — GCM provera ne radi")
	}
}

// stari, nešifrovan plain text mora da padne na Desifruj (osnova fallback logike)
func TestDesifrujPrepoznajePlainText(t *testing.T) {
	kljuc := napraviKljuc(t)
	if _, err := Desifruj("JBSWY3DPEHPK3PXP", kljuc); err == nil {
		t.Fatal("plain text je prošao kao šifrat - fallback ne bi prepoznao stare tajne")
	}
}

// pogrešna dužina ključa je greška, ne panika
func TestSifrujKratakKljuc(t *testing.T) {
	if _, err := Sifruj("x", []byte("prekratak")); err == nil {
		t.Fatal("kratak ključ nije odbijen")
	}
}
