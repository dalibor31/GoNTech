package sqlite

import (
	"context"
	"database/sql"
	"fmt"
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
	dodaj(&model.Artikal{Naziv: "A", Kolicina: 10, KolicinaMin: 5})
	dodaj(&model.Artikal{Naziv: "B", Kolicina: 2, KolicinaMin: 5})
	dodaj(&model.Artikal{Naziv: "C", Kolicina: 0, KolicinaMin: 5})

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
	if zalihe[0].Kolicina != 0 {
		t.Fatalf("prvi kritičan treba da ima količinu 0, ima %d", zalihe[0].Kolicina)
	}
}

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

// ─── stornirani prodajni nalozi ne smeju ući u prihod ────────────────────────

func TestPrihodTekuciMesec_IgnorisuSeStornirani(t *testing.T) {
	ctx := context.Background()
	baza := testDB(t)
	izv := NoviIzvestajRepo(baza)

	mustExec(t, baza, `INSERT INTO prodajni_nalozi (broj_naloga, ukupno, stornirano, datum)
		VALUES ('PR-T-001', 1000.0, 0, date('now','localtime'))`)
	mustExec(t, baza, `INSERT INTO prodajni_nalozi (broj_naloga, ukupno, stornirano, datum)
		VALUES ('PR-T-002', 500.0, 1, date('now','localtime'))`)

	prihod, err := izv.PrihodTekuciMesec(ctx)
	if err != nil {
		t.Fatalf("PrihodTekuciMesec: %v", err)
	}
	if prihod != 1000.0 {
		t.Errorf("prihod = %.2f; očekivano 1000.00 — stornirani nalog (500) ne sme da uđe", prihod)
	}
}

func TestMesecniPrihodProdaja_IgnorisuSeStornirani(t *testing.T) {
	ctx := context.Background()
	baza := testDB(t)
	izv := NoviIzvestajRepo(baza)

	mustExec(t, baza, `INSERT INTO prodajni_nalozi (broj_naloga, ukupno, stornirano, datum)
		VALUES ('PR-M-001', 2000.0, 0, date('now','localtime'))`)
	mustExec(t, baza, `INSERT INTO prodajni_nalozi (broj_naloga, ukupno, stornirano, datum)
		VALUES ('PR-M-002', 300.0, 1, date('now','localtime'))`)

	meseci, err := izv.MesecniPrihodProdaja(ctx)
	if err != nil {
		t.Fatalf("MesecniPrihodProdaja: %v", err)
	}
	if len(meseci) != 1 {
		t.Fatalf("očekivan 1 mesec, dobijeno %d", len(meseci))
	}
	if meseci[0].Iznos != 2000.0 {
		t.Errorf("iznos = %.2f; očekivano 2000.00 — stornirani (300) ne sme da uđe", meseci[0].Iznos)
	}
}

func TestPoslednjeProdaje_IgnorisuSeStornirani(t *testing.T) {
	ctx := context.Background()
	baza := testDB(t)
	izv := NoviIzvestajRepo(baza)

	mustExec(t, baza, `INSERT INTO prodajni_nalozi (broj_naloga, ukupno, stornirano, datum)
		VALUES ('PR-P-001', 100.0, 0, date('now','localtime'))`)
	mustExec(t, baza, `INSERT INTO prodajni_nalozi (broj_naloga, ukupno, stornirano, datum)
		VALUES ('PR-P-002', 200.0, 1, date('now','localtime'))`)

	lista, err := izv.PoslednjeProdaje(ctx, 10)
	if err != nil {
		t.Fatalf("PoslednjeProdaje: %v", err)
	}
	if len(lista) != 1 {
		t.Fatalf("lista ima %d redova, očekivan 1 — stornirani ne sme da uđe", len(lista))
	}
	if lista[0].BrojNaloga != "PR-P-001" {
		t.Errorf("pogrešan nalog: %s", lista[0].BrojNaloga)
	}
}

func TestTopKlijenti_IgnorisuSeStornirani(t *testing.T) {
	ctx := context.Background()
	baza := testDB(t)
	izv := NoviIzvestajRepo(baza)

	mustExec(t, baza, `INSERT INTO klijenti (ime, prezime) VALUES ('Pera', 'Perić')`)
	var klijentID int64
	if err := baza.QueryRowContext(ctx, `SELECT id FROM klijenti LIMIT 1`).Scan(&klijentID); err != nil {
		t.Fatalf("dohvati klijent id: %v", err)
	}

	mustExec(t, baza, fmt.Sprintf(
		`INSERT INTO prodajni_nalozi (broj_naloga, ukupno, stornirano, datum, klijent_id)
		 VALUES ('PR-K-001', 5000.0, 0, date('now','localtime'), %d)`, klijentID))
	mustExec(t, baza, fmt.Sprintf(
		`INSERT INTO prodajni_nalozi (broj_naloga, ukupno, stornirano, datum, klijent_id)
		 VALUES ('PR-K-002', 1000.0, 1, date('now','localtime'), %d)`, klijentID))

	top, err := izv.TopKlijenti(ctx, 5)
	if err != nil {
		t.Fatalf("TopKlijenti: %v", err)
	}
	if len(top) != 1 {
		t.Fatalf("top lista ima %d redova, očekivan 1", len(top))
	}
	if top[0].UkupnoVrednost != 5000.0 {
		t.Errorf("vrednost = %.2f; očekivano 5000.00 — stornirani (1000) ne sme da uđe", top[0].UkupnoVrednost)
	}
}

// ─── servis prihod koristi naplaceno (bruto, uključuje delove) ────────────────

func TestPrihodTekuciMesec_ServisKoristiNaplaceno(t *testing.T) {
	ctx := context.Background()
	baza := testDB(t)
	izv := NoviIzvestajRepo(baza)

	// cena_konacna=800 (neto, bez delova), naplaceno=1500 (bruto sa delovima)
	mustExec(t, baza, `INSERT INTO servisni_nalozi
		(broj_naloga, uredjaj, opis_kvara, status, cena_konacna, naplaceno, datum_prijema, datum_zavrsetka, javni_token)
		VALUES ('SR-T-001', 'Telefon', 'Kvar', 'Preuzeto', 800.0, 1500.0, date('now'), date('now','localtime'), 'tok1')`)

	prihod, err := izv.PrihodTekuciMesec(ctx)
	if err != nil {
		t.Fatalf("PrihodTekuciMesec: %v", err)
	}
	if prihod != 1500.0 {
		t.Errorf("prihod = %.2f; očekivano 1500.00 (naplaceno, ne cena_konacna 800)", prihod)
	}
}

func TestPrihodTekuciMesec_ServisNaplaceno0NeUlazi(t *testing.T) {
	ctx := context.Background()
	baza := testDB(t)
	izv := NoviIzvestajRepo(baza)

	// garancijska popravka — cena_konacna postoji ali naplaceno=0
	mustExec(t, baza, `INSERT INTO servisni_nalozi
		(broj_naloga, uredjaj, opis_kvara, status, cena_konacna, naplaceno, datum_prijema, datum_zavrsetka, javni_token)
		VALUES ('SR-T-002', 'Laptop', 'Kvar', 'Preuzeto', 1200.0, 0, date('now'), date('now','localtime'), 'tok2')`)

	prihod, err := izv.PrihodTekuciMesec(ctx)
	if err != nil {
		t.Fatalf("PrihodTekuciMesec: %v", err)
	}
	if prihod != 0.0 {
		t.Errorf("prihod = %.2f; očekivano 0 (naplaceno=0 ne sme da uđe)", prihod)
	}
}

func TestMesecniPrihodServis_KoristiNaplaceno(t *testing.T) {
	ctx := context.Background()
	baza := testDB(t)
	izv := NoviIzvestajRepo(baza)

	mustExec(t, baza, `INSERT INTO servisni_nalozi
		(broj_naloga, uredjaj, opis_kvara, status, cena_konacna, naplaceno, datum_prijema, datum_zavrsetka, javni_token)
		VALUES ('SR-M-001', 'Tablet', 'Kvar', 'Preuzeto', 500.0, 900.0, date('now'), date('now','localtime'), 'tok3')`)

	meseci, err := izv.MesecniPrihodServis(ctx)
	if err != nil {
		t.Fatalf("MesecniPrihodServis: %v", err)
	}
	if len(meseci) != 1 {
		t.Fatalf("očekivan 1 mesec, dobijeno %d", len(meseci))
	}
	if meseci[0].Iznos != 900.0 {
		t.Errorf("iznos = %.2f; očekivano 900.00 (naplaceno, ne cena_konacna 500)", meseci[0].Iznos)
	}
}

// ─── avans se uračunava u prihod (naplaceno + avans) ──────────────────────────

func TestPrihodTekuciMesec_ServisSaAvansom(t *testing.T) {
	ctx := context.Background()
	baza := testDB(t)
	izv := NoviIzvestajRepo(baza)

	// avans 400 (ranije), naplaceno 600 (ostatak pri preuzimanju) → ukupno 1000
	mustExec(t, baza, `INSERT INTO servisni_nalozi
		(broj_naloga, uredjaj, opis_kvara, status, cena_konacna, avans, naplaceno, datum_prijema, datum_zavrsetka, javni_token)
		VALUES ('SR-AV-001', 'PC', 'Kvar', 'Preuzeto', 1000.0, 400.0, 600.0, date('now'), date('now','localtime'), 'tokav1')`)

	prihod, err := izv.PrihodTekuciMesec(ctx)
	if err != nil {
		t.Fatalf("PrihodTekuciMesec: %v", err)
	}
	if prihod != 1000.0 {
		t.Errorf("prihod = %.2f; očekivano 1000.00 (naplaceno 600 + avans 400)", prihod)
	}
}

func TestPrihodTekuciMesec_ServisPotpunoAvansiran(t *testing.T) {
	ctx := context.Background()
	baza := testDB(t)
	izv := NoviIzvestajRepo(baza)

	// avans pokrio ceo iznos → naplaceno=0, ali prihod NIJE 0 (avans 1000)
	mustExec(t, baza, `INSERT INTO servisni_nalozi
		(broj_naloga, uredjaj, opis_kvara, status, cena_konacna, avans, naplaceno, datum_prijema, datum_zavrsetka, javni_token)
		VALUES ('SR-AV-002', 'Laptop', 'Kvar', 'Preuzeto', 1000.0, 1000.0, 0.0, date('now'), date('now','localtime'), 'tokav2')`)

	prihod, err := izv.PrihodTekuciMesec(ctx)
	if err != nil {
		t.Fatalf("PrihodTekuciMesec: %v", err)
	}
	if prihod != 1000.0 {
		t.Errorf("prihod = %.2f; očekivano 1000.00 (potpuno avansiran ne sme da nestane)", prihod)
	}
}

// garancija/besplatna popravka: naplaceno=0 i avans=NULL → ne ulazi u prihod
func TestPrihodTekuciMesec_GarancijaNeUlazi(t *testing.T) {
	ctx := context.Background()
	baza := testDB(t)
	izv := NoviIzvestajRepo(baza)

	mustExec(t, baza, `INSERT INTO servisni_nalozi
		(broj_naloga, uredjaj, opis_kvara, status, cena_konacna, naplaceno, datum_prijema, datum_zavrsetka, javni_token)
		VALUES ('SR-GAR-001', 'Telefon', 'Garancija', 'Preuzeto', 0.0, 0.0, date('now'), date('now','localtime'), 'tokgar')`)

	prihod, err := izv.PrihodTekuciMesec(ctx)
	if err != nil {
		t.Fatalf("PrihodTekuciMesec: %v", err)
	}
	if prihod != 0.0 {
		t.Errorf("prihod = %.2f; očekivano 0 (garancija bez naplate i avansa)", prihod)
	}
}

// ─── TopKlijenti broji samo preuzete naloge sa naplatom/avansom ───────────────

func TestTopKlijenti_SamoPreuzetiNalozi(t *testing.T) {
	ctx := context.Background()
	baza := testDB(t)
	izv := NoviIzvestajRepo(baza)

	mustExec(t, baza, `INSERT INTO klijenti (ime, prezime) VALUES ('Mika', 'Mikić')`)
	var klijentID int64
	if err := baza.QueryRowContext(ctx, `SELECT id FROM klijenti LIMIT 1`).Scan(&klijentID); err != nil {
		t.Fatalf("dohvati klijent id: %v", err)
	}

	// preuzet nalog sa naplatom 800 — broji se
	mustExec(t, baza, fmt.Sprintf(`INSERT INTO servisni_nalozi
		(broj_naloga, uredjaj, opis_kvara, status, cena_konacna, naplaceno, datum_prijema, datum_zavrsetka, javni_token, klijent_id)
		VALUES ('SR-TK-001', 'PC', 'Kvar', 'Preuzeto', 800.0, 800.0, date('now'), date('now','localtime'), 'toktk1', %d)`, klijentID))
	// nalog u popravci sa cena_konacna 5000 — NE sme da se broji (nije preuzet)
	mustExec(t, baza, fmt.Sprintf(`INSERT INTO servisni_nalozi
		(broj_naloga, uredjaj, opis_kvara, status, cena_konacna, naplaceno, datum_prijema, javni_token, klijent_id)
		VALUES ('SR-TK-002', 'Laptop', 'Kvar', 'U popravci', 5000.0, 0.0, date('now'), 'toktk2', %d)`, klijentID))

	top, err := izv.TopKlijenti(ctx, 5)
	if err != nil {
		t.Fatalf("TopKlijenti: %v", err)
	}
	if len(top) != 1 {
		t.Fatalf("top lista ima %d redova, očekivan 1", len(top))
	}
	if top[0].UkupnoVrednost != 800.0 {
		t.Errorf("vrednost = %.2f; očekivano 800.00 (samo preuzet nalog, ne 5000 u popravci)", top[0].UkupnoVrednost)
	}
}

// ─── Kombinovani test: prodaja (bez storniranih) + servis (naplaceno) ────────

func TestPrihodTekuciMesec_Kombinovani(t *testing.T) {
	ctx := context.Background()
	baza := testDB(t)
	izv := NoviIzvestajRepo(baza)

	// prodaja: 1000 normalna + 400 stornirana
	mustExec(t, baza, `INSERT INTO prodajni_nalozi (broj_naloga, ukupno, stornirano, datum)
		VALUES ('PR-C-001', 1000.0, 0, date('now','localtime'))`)
	mustExec(t, baza, `INSERT INTO prodajni_nalozi (broj_naloga, ukupno, stornirano, datum)
		VALUES ('PR-C-002', 400.0, 1, date('now','localtime'))`)

	// servis: naplaceno=600 (cena_konacna=200 neto)
	mustExec(t, baza, `INSERT INTO servisni_nalozi
		(broj_naloga, uredjaj, opis_kvara, status, cena_konacna, naplaceno, datum_prijema, datum_zavrsetka, javni_token)
		VALUES ('SR-C-001', 'PC', 'Kvar', 'Preuzeto', 200.0, 600.0, date('now'), date('now','localtime'), 'tok4')`)

	// 1000 (prodaja) + 600 (servis naplaceno) = 1600; 400 storniranih ne ulazi
	prihod, err := izv.PrihodTekuciMesec(ctx)
	if err != nil {
		t.Fatalf("PrihodTekuciMesec: %v", err)
	}
	const ocekivano = 1600.0
	if prihod != ocekivano {
		t.Errorf("prihod = %.2f; očekivano %.2f", prihod, ocekivano)
	}
}

// ─── auto-izračun cena_konacna ne sme da uključuje delove ─────────────────────
//
// Testiramo repo sloj direktno: simuliramo šta handler radi pri prelasku u
// Preuzeto kad CenaKonacna == nil — upisujemo samo dijagnostiku+radove, pa
// proveravamo da li naplaceno (= cena_konacna + delovi) nije dupliran.

func TestServisAutoIzracunCeneKonacne_BezDuplogDela(t *testing.T) {
	ctx := context.Background()
	baza := testDB(t)
	servisRepo := NoviServisRepo(baza)
	radoviRepo := NoviServisniRadoviRepo(baza)
	izv := NoviIzvestajRepo(baza)

	// kreiraj nalog bez cene_konacne
	nalogID, err := servisRepo.Kreiraj(ctx, &model.ServisniNalog{
		BrojNaloga:       "SR-BUG04",
		Uredjaj:          "Monitor",
		OpisKvara:        "Ne pali",
		Status:           "U popravci",
		CenaDijagnostike: 200.0,
	})
	if err != nil {
		t.Fatalf("Kreiraj nalog: %v", err)
	}

	// dodaj rad direktno u bazu (200 din)
	mustExec(t, baza, fmt.Sprintf(
		`INSERT INTO servis_radovi (nalog_id, naziv, kolicina, cena_komada, predlozeno)
		 VALUES (%d, 'Zamena baterijice', 1, 200.0, 0)`, nalogID))

	// dodaj deo direktno u bazu (300 din) — potreban je artikal u bazi
	mustExec(t, baza, `INSERT INTO artikli (naziv, kolicina, prodajna_cena) VALUES ('Baterija', 5, 300.0)`)
	var artID int64
	_ = baza.QueryRowContext(ctx, `SELECT id FROM artikli WHERE naziv='Baterija'`).Scan(&artID)
	mustExec(t, baza, fmt.Sprintf(
		`INSERT INTO servisni_delovi (nalog_id, artikal_id, kolicina, cena_komada, predlozeno)
		 VALUES (%d, %d, 1, 300.0, 0)`, nalogID, artID))

	// simuliramo ispravnu handler logiku:
	// cena_konacna = dijagnostika + radovi (BEZ delova)
	cenaKonacna := 200.0 + 200.0 // dijagnostika + rad
	if err := servisRepo.AzurirajCenuKonacnu(ctx, nalogID, cenaKonacna); err != nil {
		t.Fatalf("AzurirajCenuKonacnu: %v", err)
	}

	// iznos za naplatu = cena_konacna + delovi (bez dupliranja)
	iznosNaplacen := cenaKonacna + 300.0 // = 700
	if err := servisRepo.SacuvajNaplatu(ctx, nalogID, "Gotovina", iznosNaplacen); err != nil {
		t.Fatalf("SacuvajNaplatu: %v", err)
	}

	// postavi datum_zavrsetka i status Preuzeto
	if err := servisRepo.AzurirajStatus(ctx, nalogID, "Preuzeto"); err != nil {
		t.Fatalf("AzurirajStatus: %v", err)
	}

	// prihod treba da bude tačno 700 (nije 1000 = 400+300+300 duplikat)
	prihod, err := izv.PrihodTekuciMesec(ctx)
	if err != nil {
		t.Fatalf("PrihodTekuciMesec: %v", err)
	}
	const ocekivano = 700.0
	if prihod != ocekivano {
		t.Errorf("prihod = %.2f; očekivano %.2f (400 rad+dijag + 300 deo, bez duplikata)", prihod, ocekivano)
	}

	// proveri i da cena_konacna nije kontaminirana delovima
	nalog, err := servisRepo.DohvatiID(ctx, nalogID)
	if err != nil || nalog == nil {
		t.Fatalf("DohvatiID: %v", err)
	}
	if nalog.CenaKonacna == nil {
		t.Fatal("cena_konacna je nil — nije sačuvana")
	}
	if *nalog.CenaKonacna != 400.0 {
		t.Errorf("cena_konacna = %.2f; očekivano 400.00 (dijagnostika+rad, bez delova)", *nalog.CenaKonacna)
	}

	_ = radoviRepo // korišćen indirektno kroz bazu
}

// ─── helper ──────────────────────────────────────────────────────────────────

func mustExec(t *testing.T, db *sql.DB, query string) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), query); err != nil {
		t.Fatalf("mustExec: %v\nSQL: %s", err, query)
	}
}
