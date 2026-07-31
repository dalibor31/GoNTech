package sqlite

import (
	"context"
	"testing"

	"ntech/internal/model"
)

// TestServisKreiraj_IdempotencyKey: dva poziva Kreiraj sa istim IdempotencyKey
// (simulira dupliran POST — dupli klik, "Nazad" pa ponovni submit, mrežni retry)
// vraćaju ISTI nalogID i ne prave drugi nalog — isti obrazac kao za prodaju
// (v. TestProdajaKreiraj_IdempotencyKey u prodaja_kreiraj_test.go).
func TestServisKreiraj_IdempotencyKey(t *testing.T) {
	ctx := context.Background()
	baza := testDB(t)
	repo := NoviServisRepo(baza)

	id1, err := repo.Kreiraj(ctx, &model.ServisniNalog{
		Uredjaj: "Laptop", OpisKvara: "ne pali", Status: "Primljeno",
		IdempotencyKey: "test-servis-kljuc-123",
	})
	if err != nil {
		t.Fatalf("prvi Kreiraj: %v", err)
	}

	// drugi poziv sa ISTIM ključem (nov nalog, kao pri ponovljenom POST-u)
	id2, err := repo.Kreiraj(ctx, &model.ServisniNalog{
		Uredjaj: "Laptop", OpisKvara: "ne pali", Status: "Primljeno",
		IdempotencyKey: "test-servis-kljuc-123",
	})
	if err != nil {
		t.Fatalf("drugi Kreiraj (dupliran POST): %v", err)
	}

	if id1 != id2 {
		t.Errorf("drugi poziv sa istim idempotency ključem vratio drugačiji ID: %d != %d — napravljen dupli nalog", id1, id2)
	}

	var brNaloga int
	baza.QueryRowContext(ctx, "SELECT COUNT(*) FROM servisni_nalozi WHERE idempotency_key = ?", "test-servis-kljuc-123").Scan(&brNaloga)
	if brNaloga != 1 {
		t.Errorf("broj naloga sa ovim idempotency ključem = %d, očekivano 1", brNaloga)
	}
}

// TestServisKreiraj_BezIdempotencyKljuca: prazan ključ (pozivalac ga ne koristi) —
// dva odvojena poziva prave DVA odvojena naloga, kao i pre uvođenja zaštite.
func TestServisKreiraj_BezIdempotencyKljuca(t *testing.T) {
	ctx := context.Background()
	baza := testDB(t)
	repo := NoviServisRepo(baza)

	id1, err := repo.Kreiraj(ctx, &model.ServisniNalog{Uredjaj: "PC", OpisKvara: "kvar", Status: "Primljeno"})
	if err != nil {
		t.Fatalf("prvi Kreiraj: %v", err)
	}
	id2, err := repo.Kreiraj(ctx, &model.ServisniNalog{Uredjaj: "PC", OpisKvara: "kvar", Status: "Primljeno"})
	if err != nil {
		t.Fatalf("drugi Kreiraj: %v", err)
	}
	if id1 == id2 {
		t.Errorf("bez idempotency ključa očekivana dva različita naloga, dobijen isti ID %d", id1)
	}
}
