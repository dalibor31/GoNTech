package sqlite

import (
	"context"
	"testing"

	"ntech/internal/model"
)

func TestPdvStopaRepo(t *testing.T) {
	db := testDB(t)
	repo := NoviPdvStopaRepo(db)
	ctx := context.Background()

	// migracija 040 seeduje 3 stope; redosled ASC → opšta(20), posebna(10), oslobođeno(0)
	t.Run("seed iz migracije", func(t *testing.T) {
		stope, err := repo.Lista(ctx, true)
		if err != nil {
			t.Fatalf("Lista: %v", err)
		}
		if len(stope) != 3 {
			t.Fatalf("očekivano 3 seed stope, dobijeno %d", len(stope))
		}
		if stope[0].Stopa != 20 || stope[0].Oznaka != "opsta" {
			t.Errorf("prva stopa = %v %q, očekivano 20 opsta", stope[0].Stopa, stope[0].Oznaka)
		}
		if stope[2].Stopa != 0 || stope[2].Oznaka != "oslobodjeno" {
			t.Errorf("treća stopa = %v %q, očekivano 0 oslobodjeno", stope[2].Stopa, stope[2].Oznaka)
		}
	})

	var novaID int64

	t.Run("kreiraj i dohvati", func(t *testing.T) {
		id, err := repo.Kreiraj(ctx, &model.PdvStopa{
			Naziv: "Test stopa", Stopa: 11.5, Oznaka: "posebna", Aktivna: true, Redosled: 9,
		})
		if err != nil {
			t.Fatalf("Kreiraj: %v", err)
		}
		novaID = id
		s, err := repo.DohvatiID(ctx, id)
		if err != nil {
			t.Fatalf("DohvatiID: %v", err)
		}
		if s.Naziv != "Test stopa" || s.Stopa != 11.5 || s.Oznaka != "posebna" || !s.Aktivna || s.Redosled != 9 {
			t.Errorf("dohvaćeno %+v, ne poklapa se sa unetim", s)
		}
	})

	t.Run("izmeni", func(t *testing.T) {
		err := repo.Izmeni(ctx, &model.PdvStopa{
			ID: novaID, Naziv: "Izmenjena", Stopa: 12, Oznaka: "opsta", Aktivna: true, Redosled: 5,
		})
		if err != nil {
			t.Fatalf("Izmeni: %v", err)
		}
		s, err := repo.DohvatiID(ctx, novaID)
		if err != nil {
			t.Fatalf("DohvatiID posle izmene: %v", err)
		}
		if s.Naziv != "Izmenjena" || s.Stopa != 12 || s.Oznaka != "opsta" || s.Redosled != 5 {
			t.Errorf("posle izmene %+v, ne poklapa se sa izmenom", s)
		}
	})

	t.Run("arhiviranje izostavlja iz aktivnih ali zadržava u punoj listi", func(t *testing.T) {
		if err := repo.PostaviAktivnu(ctx, novaID, false); err != nil {
			t.Fatalf("PostaviAktivnu: %v", err)
		}
		aktivne, err := repo.Lista(ctx, true)
		if err != nil {
			t.Fatalf("Lista aktivne: %v", err)
		}
		for _, s := range aktivne {
			if s.ID == novaID {
				t.Errorf("arhivirana stopa %d ne sme biti među aktivnima", novaID)
			}
		}
		sve, err := repo.Lista(ctx, false)
		if err != nil {
			t.Fatalf("Lista sve: %v", err)
		}
		nadjena := false
		for _, s := range sve {
			if s.ID == novaID {
				nadjena = true
				if s.Aktivna {
					t.Errorf("stopa %d treba da bude aktivna=false posle arhiviranja", novaID)
				}
			}
		}
		if !nadjena {
			t.Errorf("arhivirana stopa %d mora ostati u punoj listi (ne briše se)", novaID)
		}
	})
}
