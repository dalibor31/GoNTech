package sqlite

import (
	"context"
	"testing"

	"ntech/internal/model"
)

func TestIzvestajArtikliBrojaci(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	art := NoviArtikalRepo(db)
	izv := NoviIzvestajRepo(db)

	dodaj := func(a *model.Artikal) {
		if _, err := art.Kreiraj(ctx, a); err != nil {
			t.Fatalf("Kreiraj: %v", err)
		}
	}
	dodaj(&model.Artikal{Naziv: "A", Kolicina: 10, KolicinMin: 5})
	dodaj(&model.Artikal{Naziv: "B", Kolicina: 2, KolicinMin: 5}) // kritičan
	dodaj(&model.Artikal{Naziv: "C", Kolicina: 0, KolicinMin: 5}) // kritičan, nula

	if n, err := izv.BrojArtikala(ctx); err != nil || n != 3 {
		t.Fatalf("BrojArtikala = %d, err=%v; očekivano 3", n, err)
	}
	if n, err := izv.BrojKriticnihZaliha(ctx); err != nil || n != 2 {
		t.Fatalf("BrojKriticnihZaliha = %d, err=%v; očekivano 2", n, err)
	}

	zalihe, err := izv.KriticneZalihe(ctx, 5)
	if err != nil {
		t.Fatalf("KriticneZalihe: %v", err)
	}
	if len(zalihe) != 2 {
		t.Fatalf("KriticneZalihe vratio %d, očekivano 2", len(zalihe))
	}
	// sortirano po količini rastuće → prvi je onaj sa 0
	if zalihe[0].Kolicina != 0 {
		t.Fatalf("prvi kritičan treba da ima količinu 0, ima %d", zalihe[0].Kolicina)
	}
}

// prazna baza — brojači 0, liste prazne, bez greške
func TestIzvestajPraznaBaza(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	izv := NoviIzvestajRepo(db)

	if n, err := izv.BrojArtikala(ctx); err != nil || n != 0 {
		t.Errorf("BrojArtikala = %d, err=%v", n, err)
	}
	if v, err := izv.PrihodTekuciMesec(ctx); err != nil || v != 0 {
		t.Errorf("PrihodTekuciMesec = %v, err=%v", v, err)
	}
	if l, err := izv.PoslednjiServisi(ctx, 5); err != nil || len(l) != 0 {
		t.Errorf("PoslednjiServisi len=%d, err=%v", len(l), err)
	}
	if l, err := izv.TopKlijenti(ctx, 10); err != nil || len(l) != 0 {
		t.Errorf("TopKlijenti len=%d, err=%v", len(l), err)
	}
	if l, err := izv.MesecniPrihodProdaja(ctx); err != nil || len(l) != 0 {
		t.Errorf("MesecniPrihodProdaja len=%d, err=%v", len(l), err)
	}
}
