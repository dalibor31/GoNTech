package sqlite

import (
	"context"
	"testing"
	"time"

	"ntech/internal/model"
)

// TestKpoRedniBroj: redni_broj mora biti dodeljen automatski i bez prekida —
// Pravilnik o KPO zahteva kontinuiran niz, i knjiga se nikad ne briše (storno
// se dograđuje kao nov red, ne zamenjuje stari).
func TestKpoRedniBroj(t *testing.T) {
	db := testDB(t)
	repo := NoviKpoRepo(db)
	ctx := context.Background()
	dp := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	for _, brojDok := range []string{"SN-1", "SN-2", "SN-3"} {
		if _, err := repo.Kreiraj(ctx, &model.KpoZapis{
			DatumPrometa: dp, BrojDokumenta: brojDok, Prihod: 100,
		}); err != nil {
			t.Fatalf("Kreiraj %s: %v", brojDok, err)
		}
	}

	zapisi, err := repo.Lista(ctx, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("Lista: %v", err)
	}
	if len(zapisi) != 3 {
		t.Fatalf("očekivano 3 zapisa, dobijeno %d", len(zapisi))
	}
	for i, z := range zapisi {
		if z.RedniBroj == nil || *z.RedniBroj != i+1 {
			t.Errorf("zapis %q: redni_broj=%v, očekivano %d", z.BrojDokumenta, z.RedniBroj, i+1)
		}
	}
}
