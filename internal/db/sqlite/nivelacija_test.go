package sqlite

import (
	"context"
	"errors"
	"testing"

	"ntech/internal/model"
)

// TestPromeniCenu_AzuriraCenuIUpisujeNivelaciju: PromeniCenu čita staru cenu iz
// baze, menja prodajnu cenu i kreira nivelacioni zapis u jednoj transakciji.
func TestPromeniCenu_AzuriraCenuIUpisujeNivelaciju(t *testing.T) {
	ctx := context.Background()
	baza := testDB(t)
	artRepo := NoviArtikalRepo(baza)
	nivRepo := NoviNivelacijaRepo(baza)

	artID, _ := artRepo.Kreiraj(ctx, &model.Artikal{Naziv: "Miš", ProdajnaCena: 1000})

	niv, err := nivRepo.PromeniCenu(ctx, artID, 1200, "sezonska korekcija", nil)
	if err != nil {
		t.Fatalf("PromeniCenu: %v", err)
	}

	// povratna vrednost
	if niv.StaraCena != 1000 {
		t.Errorf("StaraCena = %.2f, očekivano 1000", niv.StaraCena)
	}
	if niv.NovaCena != 1200 {
		t.Errorf("NovaCena = %.2f, očekivano 1200", niv.NovaCena)
	}
	if niv.Izvor != "rucno" {
		t.Errorf("Izvor = %q, očekivano \"rucno\"", niv.Izvor)
	}

	// artikal u bazi mora imati novu cenu
	a, _ := artRepo.DohvatiID(ctx, artID)
	if a.ProdajnaCena != 1200 {
		t.Errorf("prodajna_cena u bazi = %.2f, očekivano 1200", a.ProdajnaCena)
	}

	// nivelacioni zapis mora postojati u bazi
	var stara, nova float64
	var izvor string
	err = baza.QueryRowContext(ctx,
		"SELECT stara_cena, nova_cena, izvor FROM nivelacije WHERE artikal_id = ?", artID,
	).Scan(&stara, &nova, &izvor)
	if err != nil {
		t.Fatalf("nivelacioni zapis nije pronađen: %v", err)
	}
	if stara != 1000 || nova != 1200 || izvor != "rucno" {
		t.Errorf("zapis: stara=%.2f nova=%.2f izvor=%q, očekivano 1000/1200/rucno", stara, nova, izvor)
	}
}

// TestPromeniCenu_NepostojeciArtikal: pokušaj promene cene nepostojećeg artikla
// vraća ErrArtikalNePostoji, baza ostaje neizmenjena.
func TestPromeniCenu_NepostojeciArtikal(t *testing.T) {
	ctx := context.Background()
	baza := testDB(t)
	nivRepo := NoviNivelacijaRepo(baza)

	_, err := nivRepo.PromeniCenu(ctx, 9999, 500, "test", nil)
	if !errors.Is(err, ErrArtikalNePostoji) {
		t.Errorf("greška = %v, očekivano ErrArtikalNePostoji", err)
	}

	// nivelacioni zapis ne sme biti kreiran
	var br int
	baza.QueryRowContext(ctx, "SELECT COUNT(*) FROM nivelacije").Scan(&br)
	if br != 0 {
		t.Errorf("nivelacije = %d, očekivano 0 (transakcija rollback)", br)
	}
}

// TestPromeniCenu_VisePromena: svaka promena čita prethodnu cenu kao "staru".
func TestPromeniCenu_VisePromena(t *testing.T) {
	ctx := context.Background()
	baza := testDB(t)
	artRepo := NoviArtikalRepo(baza)
	nivRepo := NoviNivelacijaRepo(baza)

	artID, _ := artRepo.Kreiraj(ctx, &model.Artikal{Naziv: "Tastatura", ProdajnaCena: 800})

	nivRepo.PromeniCenu(ctx, artID, 900, "korekcija 1", nil)
	niv2, _ := nivRepo.PromeniCenu(ctx, artID, 750, "korekcija 2", nil)

	// druga promena čita 900 (ne originalni 800) kao staru cenu
	if niv2.StaraCena != 900 {
		t.Errorf("StaraCena druge promene = %.2f, očekivano 900", niv2.StaraCena)
	}

	// ukupno 2 zapisa u nivelacije
	var br int
	baza.QueryRowContext(ctx, "SELECT COUNT(*) FROM nivelacije WHERE artikal_id = ?", artID).Scan(&br)
	if br != 2 {
		t.Errorf("broj nivelacijskih zapisa = %d, očekivano 2", br)
	}
}
