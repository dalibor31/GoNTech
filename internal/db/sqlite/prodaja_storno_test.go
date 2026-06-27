package sqlite

import (
	"context"
	"testing"
	"time"

	"ntech/internal/model"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

// seedArtikalSaStanjem kreira artikal sa zadatom količinom i vraća njegov ID.
func seedArtikalSaStanjem(t *testing.T, ctx context.Context, repo *ArtikalRepo, naziv string, kolicina int) int64 {
	t.Helper()
	id, err := repo.Kreiraj(ctx, &model.Artikal{
		Naziv:    naziv,
		Kolicina: kolicina,
		Tip:      model.TipProizvod,
	})
	if err != nil {
		t.Fatalf("seedArtikalSaStanjem %q: %v", naziv, err)
	}
	return id
}

// seedUsluga kreira artikal tipa "usluga" (bez praćenja lagera).
func seedUsluga(t *testing.T, ctx context.Context, repo *ArtikalRepo, naziv string) int64 {
	t.Helper()
	id, err := repo.Kreiraj(ctx, &model.Artikal{
		Naziv: naziv,
		Tip:   model.TipUsluga,
	})
	if err != nil {
		t.Fatalf("seedUsluga %q: %v", naziv, err)
	}
	return id
}

// prodajNalog kreira prodajni nalog sa zadatim stavkama i vraća ID naloga.
func prodajNalog(t *testing.T, ctx context.Context, repo *ProdajaRepo, broj string, stavke []model.StavkaProdaje) int64 {
	t.Helper()
	ukupno := 0.0
	for _, s := range stavke {
		ukupno += float64(s.Kolicina) * s.CenaPoKomadu
	}
	id, err := repo.Kreiraj(ctx, &model.ProdajniNalog{
		BrojNaloga:    broj,
		Ukupno:        ukupno,
		NacinPlacanja: "gotovina",
		Datum:         time.Now(),
	}, stavke, nil)
	if err != nil {
		t.Fatalf("prodajNalog %q: %v", broj, err)
	}
	return id
}

// dohvatiKolicinu čita trenutnu količinu artikla iz baze.
func dohvatiKolicinu(t *testing.T, ctx context.Context, repo *ArtikalRepo, id int64) int {
	t.Helper()
	a, err := repo.DohvatiID(ctx, id)
	if err != nil || a == nil {
		t.Fatalf("dohvatiKolicinu id=%d: %v", id, err)
	}
	return a.Kolicina
}

// ─── testovi ─────────────────────────────────────────────────────────────────

// Osnovni slučaj: storno vraća tačnu količinu u magacin.
func TestStornoVracaKolicinuUMagacin(t *testing.T) {
	ctx := context.Background()
	baza := testDB(t)
	artRepo := NoviArtikalRepo(baza)
	prodRepo := NoviProdajaRepo(baza)

	artID := seedArtikalSaStanjem(t, ctx, artRepo, "Punjač", 10)

	nalogID := prodajNalog(t, ctx, prodRepo, "PR-S-001", []model.StavkaProdaje{
		{ArtikalID: artID, Kolicina: 3, CenaPoKomadu: 100},
	})

	// poslije prodaje: 10 - 3 = 7
	if got := dohvatiKolicinu(t, ctx, artRepo, artID); got != 7 {
		t.Fatalf("posle prodaje: kolicina = %d, očekivano 7", got)
	}

	if err := prodRepo.Storno(ctx, nalogID, "test storno", nil); err != nil {
		t.Fatalf("Storno: %v", err)
	}

	// posle storna: 7 + 3 = 10 (prvobitno stanje)
	if got := dohvatiKolicinu(t, ctx, artRepo, artID); got != 10 {
		t.Errorf("posle storna: kolicina = %d, očekivano 10", got)
	}
}

// Nalog sa više stavki — sve se vraćaju.
func TestStornoViseSlavki(t *testing.T) {
	ctx := context.Background()
	baza := testDB(t)
	artRepo := NoviArtikalRepo(baza)
	prodRepo := NoviProdajaRepo(baza)

	aID := seedArtikalSaStanjem(t, ctx, artRepo, "Miš", 20)
	bID := seedArtikalSaStanjem(t, ctx, artRepo, "Tastatura", 15)

	nalogID := prodajNalog(t, ctx, prodRepo, "PR-S-002", []model.StavkaProdaje{
		{ArtikalID: aID, Kolicina: 5, CenaPoKomadu: 50},
		{ArtikalID: bID, Kolicina: 3, CenaPoKomadu: 80},
	})

	if err := prodRepo.Storno(ctx, nalogID, "povrat", nil); err != nil {
		t.Fatalf("Storno: %v", err)
	}

	if got := dohvatiKolicinu(t, ctx, artRepo, aID); got != 20 {
		t.Errorf("Miš: kolicina = %d, očekivano 20", got)
	}
	if got := dohvatiKolicinu(t, ctx, artRepo, bID); got != 15 {
		t.Errorf("Tastatura: kolicina = %d, očekivano 15", got)
	}
}

// Usluga (tip != "proizvod") — storno je dozvoljen ali se količina ne vraća.
func TestStornoUslugaNeVracaKolicinu(t *testing.T) {
	ctx := context.Background()
	baza := testDB(t)
	artRepo := NoviArtikalRepo(baza)
	prodRepo := NoviProdajaRepo(baza)

	// proizvod koji ima lager
	artID := seedArtikalSaStanjem(t, ctx, artRepo, "Kabl", 10)
	// usluga koja nema lager
	uslugaID := seedUsluga(t, ctx, artRepo, "Instalacija")

	nalogID := prodajNalog(t, ctx, prodRepo, "PR-S-003", []model.StavkaProdaje{
		{ArtikalID: artID, Kolicina: 2, CenaPoKomadu: 200},
		{ArtikalID: uslugaID, Kolicina: 1, CenaPoKomadu: 500},
	})

	if err := prodRepo.Storno(ctx, nalogID, "test", nil); err != nil {
		t.Fatalf("Storno: %v", err)
	}

	// proizvod se vraća: 10 - 2 + 2 = 10
	if got := dohvatiKolicinu(t, ctx, artRepo, artID); got != 10 {
		t.Errorf("Kabl: kolicina = %d, očekivano 10", got)
	}
	// usluga nema količinu — proveri da nije pogrešno promenjena
	usluga, _ := artRepo.DohvatiID(ctx, uslugaID)
	if usluga.Kolicina != 0 {
		t.Errorf("Instalacija (usluga): kolicina = %d, očekivano 0 (usluge nemaju lager)", usluga.Kolicina)
	}
}

// Dupli storno — drugi pokušaj mora da vrati grešku.
func TestStornoVecStorniran(t *testing.T) {
	ctx := context.Background()
	baza := testDB(t)
	artRepo := NoviArtikalRepo(baza)
	prodRepo := NoviProdajaRepo(baza)

	artID := seedArtikalSaStanjem(t, ctx, artRepo, "USB hub", 5)
	nalogID := prodajNalog(t, ctx, prodRepo, "PR-S-004", []model.StavkaProdaje{
		{ArtikalID: artID, Kolicina: 1, CenaPoKomadu: 300},
	})

	if err := prodRepo.Storno(ctx, nalogID, "prvi", nil); err != nil {
		t.Fatalf("prvi Storno: %v", err)
	}

	// drugi storno mora da vrati grešku
	if err := prodRepo.Storno(ctx, nalogID, "drugi", nil); err == nil {
		t.Error("drugi Storno nije vratio grešku — trebalo je da odbije dupli storno")
	}

	// količina ne sme biti duplo vraćena: 5 - 1 + 1 = 5, ne 6
	if got := dohvatiKolicinu(t, ctx, artRepo, artID); got != 5 {
		t.Errorf("posle duplog storna: kolicina = %d, očekivano 5 (nema duplog vraćanja)", got)
	}
}

// Storno pa Obrisi — Obrisi ne sme ponovo da vrati robu (storno je već vratio).
func TestObrisiPosleStornaNeVracaDuplo(t *testing.T) {
	ctx := context.Background()
	baza := testDB(t)
	artRepo := NoviArtikalRepo(baza)
	prodRepo := NoviProdajaRepo(baza)

	artID := seedArtikalSaStanjem(t, ctx, artRepo, "Adapter", 8)
	nalogID := prodajNalog(t, ctx, prodRepo, "PR-S-005", []model.StavkaProdaje{
		{ArtikalID: artID, Kolicina: 2, CenaPoKomadu: 150},
	})

	// storno vraća 2 kom: 8 - 2 + 2 = 8
	if err := prodRepo.Storno(ctx, nalogID, "storno pre brisanja", nil); err != nil {
		t.Fatalf("Storno: %v", err)
	}

	// brisanje storniranog naloga — ne sme ponovo da vrati robu
	if err := prodRepo.Obrisi(ctx, nalogID, nil); err != nil {
		t.Fatalf("Obrisi: %v", err)
	}

	// mora ostati 8, ne 10 (8 + 2 duplo)
	if got := dohvatiKolicinu(t, ctx, artRepo, artID); got != 8 {
		t.Errorf("posle storno+obrisi: kolicina = %d, očekivano 8 (ne 10)", got)
	}
}

// Magacinska promena se beleži pri stornu (tip = PromenaPovracaj).
func TestStornoBeležiMagacinskuPromenu(t *testing.T) {
	ctx := context.Background()
	baza := testDB(t)
	artRepo := NoviArtikalRepo(baza)
	prodRepo := NoviProdajaRepo(baza)

	artID := seedArtikalSaStanjem(t, ctx, artRepo, "Monitor", 3)
	nalogID := prodajNalog(t, ctx, prodRepo, "PR-S-006", []model.StavkaProdaje{
		{ArtikalID: artID, Kolicina: 1, CenaPoKomadu: 500},
	})

	if err := prodRepo.Storno(ctx, nalogID, "povrat kupca", nil); err != nil {
		t.Fatalf("Storno: %v", err)
	}

	// proveravamo magacinsku promenu direktno u bazi
	var tipPromene string
	var promenaKolicine int
	err := baza.QueryRowContext(ctx,
		`SELECT tip_promene, promena_kolicine FROM magacinske_promene
		 WHERE artikal_id = ? AND referentni_id = ? AND tip_promene = ?`,
		artID, nalogID, model.PromenaPovracaj,
	).Scan(&tipPromene, &promenaKolicine)
	if err != nil {
		t.Fatalf("magacinska promena nije pronađena: %v", err)
	}
	if tipPromene != model.PromenaPovracaj {
		t.Errorf("tip_promene = %q, očekivano %q", tipPromene, model.PromenaPovracaj)
	}
	if promenaKolicine != 1 {
		t.Errorf("promena_kolicine = %d, očekivano 1", promenaKolicine)
	}
}

// Delimični storno: dva naloga za isti artikal — storno prvog ne dira drugi.
func TestStornoDvaOdvojenaNaloga(t *testing.T) {
	ctx := context.Background()
	baza := testDB(t)
	artRepo := NoviArtikalRepo(baza)
	prodRepo := NoviProdajaRepo(baza)

	artID := seedArtikalSaStanjem(t, ctx, artRepo, "SSD", 10)

	nalog1 := prodajNalog(t, ctx, prodRepo, "PR-S-007", []model.StavkaProdaje{
		{ArtikalID: artID, Kolicina: 3, CenaPoKomadu: 200},
	})
	prodajNalog(t, ctx, prodRepo, "PR-S-008", []model.StavkaProdaje{
		{ArtikalID: artID, Kolicina: 2, CenaPoKomadu: 200},
	})
	// posle oba: 10 - 3 - 2 = 5

	// storniramo samo prvi
	if err := prodRepo.Storno(ctx, nalog1, "greška", nil); err != nil {
		t.Fatalf("Storno nalog1: %v", err)
	}

	// 5 + 3 (vraćen nalog1) = 8; nalog2 (2 kom) ostaje prodat
	if got := dohvatiKolicinu(t, ctx, artRepo, artID); got != 8 {
		t.Errorf("posle storna prvog: kolicina = %d, očekivano 8", got)
	}
}
