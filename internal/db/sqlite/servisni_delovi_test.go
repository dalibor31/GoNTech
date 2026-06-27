package sqlite

import (
	"context"
	"testing"
)

// TestUgradiIliPotrazuj_SveUgradjeno: stanje >= tražena količina →
// sve se ugradi, magacin skinut, nedostaje=0, nema potraživanog reda.
func TestUgradiIliPotrazuj_SveUgradjeno(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	db.ExecContext(ctx, "INSERT INTO artikli (id, naziv, kolicina) VALUES (1, 'RAM', 10)")
	db.ExecContext(ctx, "INSERT INTO servisni_nalozi (id, broj_naloga, uredjaj, opis_kvara, status) VALUES (1, 'SN-1', 'PC', 'kvar', 'Primljeno')")

	repo := NoviServisniDeloviRepo(db)
	ugradjeno, nedostaje, err := repo.UgradiIliPotrazuj(ctx, 1, 1, 4, 500, nil, false)
	if err != nil {
		t.Fatalf("UgradiIliPotrazuj: %v", err)
	}
	if ugradjeno != 4 || nedostaje != 0 {
		t.Errorf("očekivano ugradjeno=4 nedostaje=0, dobijeno ugradjeno=%d nedostaje=%d", ugradjeno, nedostaje)
	}
	if stanje := skalarInt(t, db, "SELECT kolicina FROM artikli WHERE id=1"); stanje != 6 {
		t.Errorf("stanje magacina = %d, očekivano 6 (10−4)", stanje)
	}
	if k := skalarInt(t, db, "SELECT kolicina FROM servisni_delovi WHERE nalog_id=1 AND artikal_id=1"); k != 4 {
		t.Errorf("servisni_delovi kolicina = %d, očekivano 4", k)
	}
	if k := skalarInt(t, db, "SELECT COUNT(*) FROM servisni_potrazivani_delovi WHERE nalog_id=1"); k != 0 {
		t.Errorf("servisni_potrazivani_delovi = %d, očekivano 0 (nema nedostajućih)", k)
	}
}

// TestUgradiIliPotrazuj_MergePostojeceg: isti artikal dodat drugi put →
// kolicina u servisni_delovi se sabira (UPDATE), ne pravi se novi red.
func TestUgradiIliPotrazuj_MergePostojeceg(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	db.ExecContext(ctx, "INSERT INTO artikli (id, naziv, kolicina) VALUES (1, 'SSD', 10)")
	db.ExecContext(ctx, "INSERT INTO servisni_nalozi (id, broj_naloga, uredjaj, opis_kvara, status) VALUES (1, 'SN-1', 'PC', 'kvar', 'Primljeno')")

	repo := NoviServisniDeloviRepo(db)

	// prvi put: ugradi 2
	repo.UgradiIliPotrazuj(ctx, 1, 1, 2, 200, nil, false)
	// drugi put: ugradi još 3 (istog artikla)
	repo.UgradiIliPotrazuj(ctx, 1, 1, 3, 200, nil, false)

	// mora biti JEDAN red sa kolicinom 5, ne dva reda
	if redova := skalarInt(t, db, "SELECT COUNT(*) FROM servisni_delovi WHERE nalog_id=1 AND artikal_id=1"); redova != 1 {
		t.Errorf("broj redova = %d, očekivano 1 (merge, ne duplikat)", redova)
	}
	if k := skalarInt(t, db, "SELECT kolicina FROM servisni_delovi WHERE nalog_id=1 AND artikal_id=1"); k != 5 {
		t.Errorf("kolicina = %d, očekivano 5 (2+3)", k)
	}
	// magacin: 10 − 2 − 3 = 5
	if stanje := skalarInt(t, db, "SELECT kolicina FROM artikli WHERE id=1"); stanje != 5 {
		t.Errorf("stanje magacina = %d, očekivano 5", stanje)
	}
}

// TestUgradiIliPotrazuj_Predlozeno: predlozeno=true → ide kao predlog servisu
// (ne skida sa lagera, ugradjeno=0, nedostaje=tražena kolicina).
func TestUgradiIliPotrazuj_Predlozeno(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	db.ExecContext(ctx, "INSERT INTO artikli (id, naziv, kolicina) VALUES (1, 'GPU', 5)")
	db.ExecContext(ctx, "INSERT INTO servisni_nalozi (id, broj_naloga, uredjaj, opis_kvara, status) VALUES (1, 'SN-1', 'PC', 'kvar', 'U dijagnostici')")

	repo := NoviServisniDeloviRepo(db)
	ugradjeno, nedostaje, err := repo.UgradiIliPotrazuj(ctx, 1, 1, 3, 1000, nil, true)
	if err != nil {
		t.Fatalf("UgradiIliPotrazuj (predlozeno): %v", err)
	}
	if ugradjeno != 0 || nedostaje != 3 {
		t.Errorf("očekivano ugradjeno=0 nedostaje=3, dobijeno ugradjeno=%d nedostaje=%d", ugradjeno, nedostaje)
	}
	// magacin se NE sme dirljati — ostaje 5
	if stanje := skalarInt(t, db, "SELECT kolicina FROM artikli WHERE id=1"); stanje != 5 {
		t.Errorf("stanje magacina = %d, očekivano 5 (predlog ne skida robu)", stanje)
	}
	// u servisni_delovi ne sme biti ništa
	if k := skalarInt(t, db, "SELECT COUNT(*) FROM servisni_delovi WHERE nalog_id=1"); k != 0 {
		t.Errorf("servisni_delovi = %d, očekivano 0 (predlog ne ugrađuje)", k)
	}
	// u potrazivani_delovi mora biti red sa predlozeno=1
	if k := skalarInt(t, db, "SELECT kolicina FROM servisni_potrazivani_delovi WHERE nalog_id=1 AND predlozeno=1"); k != 3 {
		t.Errorf("potrazivani kolicina = %d, očekivano 3", k)
	}
}

// TestUgradiIliPotrazuj_NemaNaStanju: stanje=0 →
// ugradjeno=0, sve ide u potraživane, magacin ostaje 0.
func TestUgradiIliPotrazuj_NemaNaStanju(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	db.ExecContext(ctx, "INSERT INTO artikli (id, naziv, kolicina) VALUES (1, 'Ekran', 0)")
	db.ExecContext(ctx, "INSERT INTO servisni_nalozi (id, broj_naloga, uredjaj, opis_kvara, status) VALUES (1, 'SN-1', 'Telefon', 'pukao ekran', 'Primljeno')")

	repo := NoviServisniDeloviRepo(db)
	ugradjeno, nedostaje, err := repo.UgradiIliPotrazuj(ctx, 1, 1, 2, 3000, nil, false)
	if err != nil {
		t.Fatalf("UgradiIliPotrazuj: %v", err)
	}
	if ugradjeno != 0 || nedostaje != 2 {
		t.Errorf("očekivano ugradjeno=0 nedostaje=2, dobijeno ugradjeno=%d nedostaje=%d", ugradjeno, nedostaje)
	}
	// magacin ne sme biti negativan
	if stanje := skalarInt(t, db, "SELECT kolicina FROM artikli WHERE id=1"); stanje != 0 {
		t.Errorf("stanje magacina = %d, očekivano 0 (ne ide u minus)", stanje)
	}
	// potraživani red kreiran sa kolicinom 2
	if k := skalarInt(t, db, "SELECT kolicina FROM servisni_potrazivani_delovi WHERE nalog_id=1 AND predlozeno=0"); k != 2 {
		t.Errorf("potrazivani kolicina = %d, očekivano 2", k)
	}
	// u servisni_delovi nema ništa
	if k := skalarInt(t, db, "SELECT COUNT(*) FROM servisni_delovi WHERE nalog_id=1"); k != 0 {
		t.Errorf("servisni_delovi = %d, očekivano 0", k)
	}
}

// TestUgradiIliPotrazuj_MergePotrazivanihDelova: isti artikal nedostaje dva puta →
// kolicina u servisni_potrazivani_delovi se sabira, ne pravi se novi red.
func TestUgradiIliPotrazuj_MergePotrazivanihDelova(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	db.ExecContext(ctx, "INSERT INTO artikli (id, naziv, kolicina) VALUES (1, 'Baterija', 0)")
	db.ExecContext(ctx, "INSERT INTO servisni_nalozi (id, broj_naloga, uredjaj, opis_kvara, status) VALUES (1, 'SN-1', 'Telefon', 'ne puni', 'Primljeno')")

	repo := NoviServisniDeloviRepo(db)
	// prvi zahtev za 2 — sve ide u potraživane
	repo.UgradiIliPotrazuj(ctx, 1, 1, 2, 800, nil, false)
	// drugi zahtev za još 1 — treba merge, ne novi red
	repo.UgradiIliPotrazuj(ctx, 1, 1, 1, 800, nil, false)

	if redova := skalarInt(t, db, "SELECT COUNT(*) FROM servisni_potrazivani_delovi WHERE nalog_id=1 AND predlozeno=0"); redova != 1 {
		t.Errorf("broj redova = %d, očekivano 1 (merge, ne duplikat)", redova)
	}
	if k := skalarInt(t, db, "SELECT kolicina FROM servisni_potrazivani_delovi WHERE nalog_id=1 AND predlozeno=0"); k != 3 {
		t.Errorf("kolicina = %d, očekivano 3 (2+1)", k)
	}
}
