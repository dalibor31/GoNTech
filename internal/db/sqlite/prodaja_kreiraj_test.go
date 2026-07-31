package sqlite

import (
	"context"
	"testing"
	"time"

	"ntech/internal/model"
)

// TestProdajaKreiraj_UslugaNeSkidaLager: stavka tipa "usluga" ne prati lager —
// količina artikla se ne menja, nema magacinske promene.
func TestProdajaKreiraj_UslugaNeSkidaLager(t *testing.T) {
	ctx := context.Background()
	baza := testDB(t)
	artRepo := NoviArtikalRepo(baza)
	prodRepo := NoviProdajaRepo(baza)

	uslugaID, err := artRepo.Kreiraj(ctx, &model.Artikal{Naziv: "Dijagnostika", Tip: model.TipUsluga})
	if err != nil {
		t.Fatalf("Kreiraj usluga: %v", err)
	}

	_, err = prodRepo.Kreiraj(ctx, &model.ProdajniNalog{
		BrojNaloga: "PR-U-001", Ukupno: 500, NacinPlacanja: "gotovina", Datum: time.Now(),
	}, []model.StavkaProdaje{
		{ArtikalID: uslugaID, Kolicina: 1, CenaPoKomadu: 500},
	}, nil)
	if err != nil {
		t.Fatalf("Kreiraj prodaja: %v", err)
	}

	// količina usluge ostaje 0 — nije praćena
	a, _ := artRepo.DohvatiID(ctx, uslugaID)
	if a.Kolicina != 0 {
		t.Errorf("usluga kolicina = %d, očekivano 0 (usluge ne prate lager)", a.Kolicina)
	}
	// nema magacinske promene za uslugu
	var br int
	baza.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM magacinske_promene WHERE artikal_id = ?", uslugaID,
	).Scan(&br)
	if br != 0 {
		t.Errorf("magacinske_promene = %d, očekivano 0 za uslugu", br)
	}
}

// TestProdajaKreiraj_NedovoljnoStanja: tražena kolicina > stanje →
// vraća ErrNedovoljnoKolicine, nalog se ne kreira, stanje ne menja.
func TestProdajaKreiraj_NedovoljnoStanja(t *testing.T) {
	ctx := context.Background()
	baza := testDB(t)
	artRepo := NoviArtikalRepo(baza)
	prodRepo := NoviProdajaRepo(baza)

	artID, _ := artRepo.Kreiraj(ctx, &model.Artikal{
		Naziv: "Monitor", Tip: model.TipProizvod, Kolicina: 2,
	})

	_, err := prodRepo.Kreiraj(ctx, &model.ProdajniNalog{
		BrojNaloga: "PR-N-001", Ukupno: 1000, NacinPlacanja: "gotovina", Datum: time.Now(),
	}, []model.StavkaProdaje{
		{ArtikalID: artID, Kolicina: 5, CenaPoKomadu: 200}, // traži 5, ima 2
	}, nil)

	if err == nil {
		t.Fatal("očekivana greška ErrNedovoljnoKolicine, nije vraćena")
	}

	// stanje ostaje nepromenjeno
	a, _ := artRepo.DohvatiID(ctx, artID)
	if a.Kolicina != 2 {
		t.Errorf("stanje = %d, očekivano 2 (transakcija rollback)", a.Kolicina)
	}
	// nalog nije kreiran
	var brNaloga int
	baza.QueryRowContext(ctx, "SELECT COUNT(*) FROM prodajni_nalozi WHERE broj_naloga='PR-N-001'").Scan(&brNaloga)
	if brNaloga != 0 {
		t.Errorf("nalog kreiran uprkos grešci — očekivano 0 naloga")
	}
}

// TestProdajaKreiraj_PdvAutoKalkulacija: kada CenaBezPdv=0 a PdvStopa>0,
// cena_bez_pdv i pdv_iznos se automatski izdvajaju naniže iz bruto cene
// (CenaPoKomadu je kod punog PDV obveznika cena za naplatu, sa PDV-om).
func TestProdajaKreiraj_PdvAutoKalkulacija(t *testing.T) {
	ctx := context.Background()
	baza := testDB(t)
	artRepo := NoviArtikalRepo(baza)
	prodRepo := NoviProdajaRepo(baza)

	artID, _ := artRepo.Kreiraj(ctx, &model.Artikal{
		Naziv: "Tastatura", Tip: model.TipProizvod, Kolicina: 10,
	})

	// bruto cena 1200, PDV 20% → cena_bez_pdv = 1000, pdv_iznos = 200
	stavke := []model.StavkaProdaje{
		{ArtikalID: artID, Kolicina: 1, CenaPoKomadu: 1200, PdvStopa: 20},
		// CenaBezPdv=0 namerno — treba da se auto-izračuna
	}
	// BrojNaloga se namerno NE prosleđuje — Kreiraj ga uvek sam generiše
	// (broj_naloga se ne veruje pozivaocu, vidi ProdajaRepo.Kreiraj)
	nalogID, err := prodRepo.Kreiraj(ctx, &model.ProdajniNalog{
		Ukupno: 1200, NacinPlacanja: "gotovina", Datum: time.Now(),
	}, stavke, nil)
	if err != nil {
		t.Fatalf("Kreiraj: %v", err)
	}

	var cenaBezPdv, pdvIznos float64
	baza.QueryRowContext(ctx,
		"SELECT cena_bez_pdv, pdv_iznos FROM stavke_prodaje WHERE nalog_id = ?", nalogID,
	).Scan(&cenaBezPdv, &pdvIznos)

	const ocekivanaNeto = 1000.0
	const ocekivanPdv = 200.0
	if absF(cenaBezPdv-ocekivanaNeto) > 0.01 {
		t.Errorf("cena_bez_pdv = %.4f, očekivano %.2f", cenaBezPdv, ocekivanaNeto)
	}
	if absF(pdvIznos-ocekivanPdv) > 0.01 {
		t.Errorf("pdv_iznos = %.4f, očekivano %.2f", pdvIznos, ocekivanPdv)
	}
}

// TestProdajaKreiraj_IdempotencyKey: dva poziva Kreiraj sa istim IdempotencyKey
// (simulira dupliran POST — dupli klik, "Nazad" pa ponovni submit, mrežni retry)
// vraćaju ISTI nalogID, ne prave drugi nalog i ne skidaju stanje dvaput.
func TestProdajaKreiraj_IdempotencyKey(t *testing.T) {
	ctx := context.Background()
	baza := testDB(t)
	artRepo := NoviArtikalRepo(baza)
	prodRepo := NoviProdajaRepo(baza)

	artID, _ := artRepo.Kreiraj(ctx, &model.Artikal{
		Naziv: "Slušalice", Tip: model.TipProizvod, Kolicina: 10,
	})

	nalog := &model.ProdajniNalog{
		Ukupno: 1000, NacinPlacanja: "gotovina", Datum: time.Now(),
		IdempotencyKey: "test-kljuc-123",
	}
	stavke := []model.StavkaProdaje{
		{ArtikalID: artID, Kolicina: 2, CenaPoKomadu: 500},
	}

	id1, err := prodRepo.Kreiraj(ctx, nalog, stavke, nil)
	if err != nil {
		t.Fatalf("prvi Kreiraj: %v", err)
	}

	// drugi poziv sa ISTIM ključem (nov nalog/stavke, kao pri ponovljenom POST-u)
	id2, err := prodRepo.Kreiraj(ctx, &model.ProdajniNalog{
		Ukupno: 1000, NacinPlacanja: "gotovina", Datum: time.Now(),
		IdempotencyKey: "test-kljuc-123",
	}, []model.StavkaProdaje{
		{ArtikalID: artID, Kolicina: 2, CenaPoKomadu: 500},
	}, nil)
	if err != nil {
		t.Fatalf("drugi Kreiraj (dupliran POST): %v", err)
	}

	if id1 != id2 {
		t.Errorf("drugi poziv sa istim idempotency ključem vratio drugačiji ID: %d != %d — napravljen dupli nalog", id1, id2)
	}

	var brNaloga int
	baza.QueryRowContext(ctx, "SELECT COUNT(*) FROM prodajni_nalozi WHERE idempotency_key = ?", "test-kljuc-123").Scan(&brNaloga)
	if brNaloga != 1 {
		t.Errorf("broj naloga sa ovim idempotency ključem = %d, očekivano 1", brNaloga)
	}

	// stanje skinuto SAMO jednom (2 kom), ne dvaput (4 kom)
	a, _ := artRepo.DohvatiID(ctx, artID)
	if a.Kolicina != 8 {
		t.Errorf("stanje = %d, očekivano 8 (10-2, skinuto samo jednom)", a.Kolicina)
	}
}

func absF(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
