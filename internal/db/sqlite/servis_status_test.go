package sqlite

import (
	"context"
	"testing"

	"ntech/internal/model"
)

// TestAzurirajStatus_DatumZavrsetka: datum_zavrsetka se setuje samo kad
// nalog prelazi u Završeno ili Preuzeto, a ne za ostale statuse.
func TestAzurirajStatus_DatumZavrsetka(t *testing.T) {
	ctx := context.Background()
	baza := testDB(t)
	repo := NoviServisRepo(baza)

	nalogID, _ := repo.Kreiraj(ctx, &model.ServisniNalog{
		BrojNaloga: "SN-ST-001", Uredjaj: "PC", OpisKvara: "kvar", Status: "Primljeno",
	})

	// prelazak u "U popravci" — datum_zavrsetka ne sme biti postavljen
	repo.AzurirajStatus(ctx, nalogID, "U popravci")
	nalog, _ := repo.DohvatiID(ctx, nalogID)
	if nalog.DatumZavrsetka != nil {
		t.Errorf("U popravci: datum_zavrsetka = %v, očekivano nil", nalog.DatumZavrsetka)
	}

	// prelazak u "Završeno" — datum_zavrsetka se postavlja
	repo.AzurirajStatus(ctx, nalogID, "Završeno")
	nalog, _ = repo.DohvatiID(ctx, nalogID)
	if nalog.DatumZavrsetka == nil {
		t.Error("Završeno: datum_zavrsetka je nil, očekivano da bude postavljen")
	}

	// drugi prelazak u "Preuzeto" — datum_zavrsetka se čuva (COALESCE, ne menja)
	prvoDatum := nalog.DatumZavrsetka
	repo.AzurirajStatus(ctx, nalogID, "Preuzeto")
	nalog, _ = repo.DohvatiID(ctx, nalogID)
	if nalog.DatumZavrsetka == nil {
		t.Error("Preuzeto: datum_zavrsetka je nil")
	}
	if !nalog.DatumZavrsetka.Equal(*prvoDatum) {
		t.Errorf("datum_zavrsetka promenjen: bio %v, sada %v (COALESCE treba da čuva original)", prvoDatum, nalog.DatumZavrsetka)
	}
}

// TestAzurirajStatus_ResetPopravkaOdbijena: svaka promena statusa resetuje
// popravka_odbijena na 0, čak i ako je pre bila 1.
func TestAzurirajStatus_ResetPopravkaOdbijena(t *testing.T) {
	ctx := context.Background()
	baza := testDB(t)
	repo := NoviServisRepo(baza)

	nalogID, _ := repo.Kreiraj(ctx, &model.ServisniNalog{
		BrojNaloga: "SN-ST-002", Uredjaj: "Laptop", OpisKvara: "kvar", Status: "U dijagnostici",
	})

	// postavi popravka_odbijena=1 direktno
	baza.ExecContext(ctx, "UPDATE servisni_nalozi SET popravka_odbijena=1 WHERE id=?", nalogID)

	nalog, _ := repo.DohvatiID(ctx, nalogID)
	if !nalog.PopravkaOdbijena {
		t.Fatal("setup: popravka_odbijena treba da bude true")
	}

	// promena statusa → reset
	repo.AzurirajStatus(ctx, nalogID, "U popravci")
	nalog, _ = repo.DohvatiID(ctx, nalogID)
	if nalog.PopravkaOdbijena {
		t.Error("posle AzurirajStatus: popravka_odbijena treba da bude false")
	}
}

// TestOdbijPopravku: status → Završeno, popravka_odbijena=1, datum_zavrsetka postavljen.
func TestOdbijPopravku(t *testing.T) {
	ctx := context.Background()
	baza := testDB(t)
	repo := NoviServisRepo(baza)

	nalogID, _ := repo.Kreiraj(ctx, &model.ServisniNalog{
		BrojNaloga: "SN-ST-003", Uredjaj: "Telefon", OpisKvara: "razbijen ekran", Status: "U dijagnostici",
	})

	const cenaDijag = 1500.0
	if err := repo.OdbijPopravku(ctx, nalogID, cenaDijag); err != nil {
		t.Fatalf("OdbijPopravku: %v", err)
	}

	nalog, _ := repo.DohvatiID(ctx, nalogID)
	if nalog.Status != model.StatusZavrseno {
		t.Errorf("status = %q, očekivano %q", nalog.Status, model.StatusZavrseno)
	}
	if !nalog.PopravkaOdbijena {
		t.Error("popravka_odbijena treba da bude true")
	}
	if nalog.DatumZavrsetka == nil {
		t.Error("datum_zavrsetka treba da bude postavljen")
	}
	if nalog.CenaDijagnostike != cenaDijag {
		t.Errorf("cena_dijagnostike = %.2f, očekivano %.2f", nalog.CenaDijagnostike, cenaDijag)
	}
}
