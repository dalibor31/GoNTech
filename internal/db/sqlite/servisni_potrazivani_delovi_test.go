package sqlite

import (
	"context"
	"database/sql"
	"testing"
)

// TestPotrazivaniTok pokriva ceo tok: ugradnja sa manjkom → potraživani deo →
// dolazak robe → pokrivanje (prebacivanje u ugrađene, skidanje magacina, otključavanje).
func TestPotrazivaniTok(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	// artikal sa stanjem 2, nalog u statusu Primljeno
	if _, err := db.ExecContext(ctx, "INSERT INTO artikli (id, naziv, kolicina) VALUES (1, 'Test CPU', 2)"); err != nil {
		t.Fatalf("insert artikal: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		"INSERT INTO servisni_nalozi (id, broj_naloga, uredjaj, opis_kvara, status) VALUES (1, 'SN-1', 'PC', 'kvar', 'Primljeno')",
	); err != nil {
		t.Fatalf("insert nalog: %v", err)
	}

	deloviRepo := NoviServisniDeloviRepo(db)
	potrRepo := NoviServisniPotrazivaniDeloviRepo(db)

	// traži 5, na stanju 2 → ugradi 2, nedostaje 3
	ugradjeno, nedostaje, err := deloviRepo.UgradiIliPotrazuj(ctx, 1, 1, 5, 100, nil)
	if err != nil {
		t.Fatalf("UgradiIliPotrazuj: %v", err)
	}
	if ugradjeno != 2 || nedostaje != 3 {
		t.Fatalf("očekivano ugradjeno=2 nedostaje=3, dobijeno ugradjeno=%d nedostaje=%d", ugradjeno, nedostaje)
	}

	// stanje magacina mora biti 0 (skinuto 2), NE u minus
	if stanje := skalarInt(t, db, "SELECT kolicina FROM artikli WHERE id=1"); stanje != 0 {
		t.Fatalf("posle ugradnje stanje očekivano 0, dobijeno %d", stanje)
	}
	// ugrađeni deo: 2 kom; potraživani: 3 kom
	if k := skalarInt(t, db, "SELECT kolicina FROM servisni_delovi WHERE nalog_id=1 AND artikal_id=1"); k != 2 {
		t.Fatalf("ugrađeni deo očekivano 2, dobijeno %d", k)
	}
	if k := skalarInt(t, db, "SELECT kolicina FROM servisni_potrazivani_delovi WHERE nalog_id=1 AND artikal_id=1"); k != 3 {
		t.Fatalf("potraživani očekivano 3, dobijeno %d", k)
	}

	// stigla roba: nabavka 5 kom → stanje 0+5=5
	if _, err := db.ExecContext(ctx, "UPDATE artikli SET kolicina = kolicina + 5 WHERE id=1"); err != nil {
		t.Fatalf("nabavka: %v", err)
	}

	// pokrivanje
	otkljucani, err := potrRepo.ProveriIPocistiZaArtikal(ctx, 1)
	if err != nil {
		t.Fatalf("ProveriIPocistiZaArtikal: %v", err)
	}
	if len(otkljucani) != 1 || otkljucani[0] != 1 {
		t.Fatalf("očekivan otključan nalog [1], dobijeno %v", otkljucani)
	}

	// ugrađeni deo sada 5 kom (2 + 3 pokriveno)
	if k := skalarInt(t, db, "SELECT kolicina FROM servisni_delovi WHERE nalog_id=1 AND artikal_id=1"); k != 5 {
		t.Fatalf("ugrađeni deo posle pokrivanja očekivano 5, dobijeno %d", k)
	}
	// potraživanih više nema
	if k := skalarInt(t, db, "SELECT COUNT(*) FROM servisni_potrazivani_delovi WHERE nalog_id=1"); k != 0 {
		t.Fatalf("potraživani treba da su prazni, ostalo %d", k)
	}
	// stanje magacina: 5 − 3 (skinuto pri pokrivanju) = 2
	if stanje := skalarInt(t, db, "SELECT kolicina FROM artikli WHERE id=1"); stanje != 2 {
		t.Fatalf("stanje posle pokrivanja očekivano 2, dobijeno %d", stanje)
	}
}

// TestPotrazivaniDelimicnoPokrivanje: stanje (1) manje od potraživanog (3) —
// povuče se samo dostupno, ostatak ostaje na čekanju, nalog ostaje zaključan.
func TestPotrazivaniDelimicnoPokrivanje(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	if _, err := db.ExecContext(ctx, "INSERT INTO artikli (id, naziv, kolicina) VALUES (1, 'Test CPU', 0)"); err != nil {
		t.Fatalf("insert artikal: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		"INSERT INTO servisni_nalozi (id, broj_naloga, uredjaj, opis_kvara, status) VALUES (1, 'SN-1', 'PC', 'kvar', 'Čeka delove')",
	); err != nil {
		t.Fatalf("insert nalog: %v", err)
	}
	// ugrađeno 2, potraživano 3 (kao da je traženo 5 a bilo 2 na stanju)
	if _, err := db.ExecContext(ctx, "INSERT INTO servisni_delovi (nalog_id, artikal_id, kolicina, cena_komada) VALUES (1, 1, 2, 100)"); err != nil {
		t.Fatalf("insert deo: %v", err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO servisni_potrazivani_delovi (nalog_id, artikal_id, kolicina, cena_komada) VALUES (1, 1, 3, 100)"); err != nil {
		t.Fatalf("insert potraživani: %v", err)
	}

	// stigao samo 1 komad
	if _, err := db.ExecContext(ctx, "UPDATE artikli SET kolicina = 1 WHERE id=1"); err != nil {
		t.Fatalf("dopuna: %v", err)
	}

	potrRepo := NoviServisniPotrazivaniDeloviRepo(db)
	otkljucani, err := potrRepo.ProveriIPocistiZaArtikal(ctx, 1)
	if err != nil {
		t.Fatalf("ProveriIPocistiZaArtikal: %v", err)
	}
	// nalog se NE otključava jer još fali 2
	if len(otkljucani) != 0 {
		t.Fatalf("nalog ne sme da se otključa, dobijeno %v", otkljucani)
	}
	// ugrađeno: 2 + 1 = 3
	if k := skalarInt(t, db, "SELECT kolicina FROM servisni_delovi WHERE nalog_id=1 AND artikal_id=1"); k != 3 {
		t.Fatalf("ugrađeni deo očekivano 3, dobijeno %d", k)
	}
	// potraživano: 3 − 1 = 2
	if k := skalarInt(t, db, "SELECT kolicina FROM servisni_potrazivani_delovi WHERE nalog_id=1 AND artikal_id=1"); k != 2 {
		t.Fatalf("potraživani očekivano 2, dobijeno %d", k)
	}
	// stanje magacina: 1 − 1 = 0
	if k := skalarInt(t, db, "SELECT kolicina FROM artikli WHERE id=1"); k != 0 {
		t.Fatalf("stanje očekivano 0, dobijeno %d", k)
	}
}

// skalarInt vraća jednu celobrojnu vrednost iz upita (za proveru stanja u testu)
func skalarInt(t *testing.T, db *sql.DB, upit string) int {
	t.Helper()
	var v int
	if err := db.QueryRow(upit).Scan(&v); err != nil {
		t.Fatalf("skalarInt %q: %v", upit, err)
	}
	return v
}
