package sqlite

import (
	"context"
	"testing"
	"time"

	"ntech/internal/model"
)

// istiDan poredi datume po godini/mesecu/danu (izbegava razlike u vremenskoj zoni/času)
func istiDan(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func TestPdvKirRepo(t *testing.T) {
	db := testDB(t)
	repo := NoviPdvKirRepo(db)
	ctx := context.Background()

	dp := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	dk := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)

	id, err := repo.Kreiraj(ctx, &model.PdvKir{
		DatumPrometa: dp, DatumKnjizenja: dk, BrojDokumenta: "R-1",
		KupacNaziv: "Kupac doo", KupacPib: "123456789", KupacMesto: "Niš",
		OsnovicaOpsta: 100, PdvOpsta: 20, Ukupno: 120,
	})
	if err != nil {
		t.Fatalf("Kreiraj: %v", err)
	}

	t.Run("datum i iznosi round-trip", func(t *testing.T) {
		k, err := repo.DohvatiID(ctx, id)
		if err != nil {
			t.Fatalf("DohvatiID: %v", err)
		}
		if !istiDan(k.DatumPrometa, dp) {
			t.Errorf("datum_prometa = %v, očekivano %v", k.DatumPrometa, dp)
		}
		if !istiDan(k.DatumKnjizenja, dk) {
			t.Errorf("datum_knjizenja = %v, očekivano %v", k.DatumKnjizenja, dk)
		}
		if k.KupacNaziv != "Kupac doo" || k.KupacPib != "123456789" || k.KupacMesto != "Niš" {
			t.Errorf("kupac podaci ne odgovaraju: %+v", k)
		}
		if k.OsnovicaOpsta != 100 || k.PdvOpsta != 20 || k.Ukupno != 120 {
			t.Errorf("iznosi ne odgovaraju: %+v", k)
		}
	})

	t.Run("filter perioda", func(t *testing.T) {
		uPeriodu, err := repo.Lista(ctx, time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC))
		if err != nil {
			t.Fatalf("Lista u periodu: %v", err)
		}
		if len(uPeriodu) != 1 {
			t.Errorf("u junu očekivano 1 zapis, dobijeno %d", len(uPeriodu))
		}
		vanPerioda, err := repo.Lista(ctx, time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC))
		if err != nil {
			t.Fatalf("Lista van perioda: %v", err)
		}
		if len(vanPerioda) != 0 {
			t.Errorf("u julu očekivano 0 zapisa, dobijeno %d", len(vanPerioda))
		}
	})

	t.Run("brisanje", func(t *testing.T) {
		if err := repo.Obrisi(ctx, id); err != nil {
			t.Fatalf("Obrisi: %v", err)
		}
		sve, _ := repo.Lista(ctx, time.Time{}, time.Time{})
		if len(sve) != 0 {
			t.Errorf("posle brisanja očekivano 0, dobijeno %d", len(sve))
		}
	})
}

func TestPdvKprRepo(t *testing.T) {
	db := testDB(t)
	repo := NoviPdvKprRepo(db)
	ctx := context.Background()

	dp := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	dpl := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)

	t.Run("sa datumom plaćanja", func(t *testing.T) {
		id, err := repo.Kreiraj(ctx, &model.PdvKpr{
			DatumPrometa: dp, DatumKnjizenja: dp, DatumPlacanja: &dpl, BrojDokumenta: "U-1",
			DobavljacNaziv: "Dobavljač doo", OsnovicaOpsta: 200, PdvOpsta: 40, Ukupno: 240,
		})
		if err != nil {
			t.Fatalf("Kreiraj: %v", err)
		}
		k, err := repo.DohvatiID(ctx, id)
		if err != nil {
			t.Fatalf("DohvatiID: %v", err)
		}
		if k.DatumPlacanja == nil {
			t.Fatalf("datum_placanja je nil, očekivan %v", dpl)
		}
		if !istiDan(*k.DatumPlacanja, dpl) {
			t.Errorf("datum_placanja = %v, očekivano %v", *k.DatumPlacanja, dpl)
		}
		if k.OsnovicaOpsta != 200 || k.PdvOpsta != 40 {
			t.Errorf("iznosi ne odgovaraju: %+v", k)
		}
	})

	t.Run("bez datuma plaćanja (NULL)", func(t *testing.T) {
		id, err := repo.Kreiraj(ctx, &model.PdvKpr{
			DatumPrometa: dp, DatumKnjizenja: dp, DatumPlacanja: nil, BrojDokumenta: "U-2",
			DobavljacNaziv: "Dobavljač 2", OsnovicaPosebna: 50, PdvPosebna: 5, Ukupno: 55,
		})
		if err != nil {
			t.Fatalf("Kreiraj: %v", err)
		}
		k, err := repo.DohvatiID(ctx, id)
		if err != nil {
			t.Fatalf("DohvatiID: %v", err)
		}
		if k.DatumPlacanja != nil {
			t.Errorf("datum_placanja = %v, očekivan nil", *k.DatumPlacanja)
		}
	})
}
