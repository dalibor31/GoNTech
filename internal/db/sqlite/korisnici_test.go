package sqlite

import (
	"context"
	"crypto/rand"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

// testDB otvara privremenu SQLite bazu i primenjuje sve migracije.
// Migracije se čitaju iz korena repozitorijuma (../../.. u odnosu na ovaj paket).
func testDB(t *testing.T) *sql.DB {
	t.Helper()
	putanja := filepath.Join(t.TempDir(), "test.db")
	db, err := OtvoriDB(putanja)
	if err != nil {
		t.Fatalf("OtvoriDB: %v", err)
	}
	if err := PokreniMigracije(db, os.DirFS("../../..")); err != nil {
		t.Fatalf("migracije: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func testKljuc(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return k
}

// sirovaTotpTajna čita kolonu totp_tajna direktno (kakva stoji u bazi)
func sirovaTotpTajna(t *testing.T, db *sql.DB, id int64) string {
	t.Helper()
	var s sql.NullString
	if err := db.QueryRow(`SELECT totp_tajna FROM korisnici WHERE id = ?`, id).Scan(&s); err != nil {
		t.Fatalf("čitanje sirove tajne: %v", err)
	}
	return s.String
}

// TOTP tajna se u bazi čuva šifrovana, a pri čitanju vraća kao čist tekst
func TestKorisniciTotpSifrovanUMirovanju(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	repo := NoviKorisniciRepo(db, testKljuc(t))

	k, err := repo.Kreiraj(ctx, "pera", "hash", "radnik")
	if err != nil {
		t.Fatalf("Kreiraj: %v", err)
	}

	const tajna = "JBSWY3DPEHPK3PXP"
	if err := repo.SacuvajTotpTajnu(ctx, k.ID, tajna); err != nil {
		t.Fatalf("SacuvajTotpTajnu: %v", err)
	}

	// u bazi NIJE čist tekst
	if sirova := sirovaTotpTajna(t, db, k.ID); sirova == tajna || sirova == "" {
		t.Fatalf("tajna u bazi nije šifrovana (sirovo = %q)", sirova)
	}

	// čitanje kroz repo vraća dešifrovanu (originalnu) tajnu
	svezi, err := repo.DohvatiPoID(ctx, k.ID)
	if err != nil {
		t.Fatalf("DohvatiPoID: %v", err)
	}
	if svezi.TotpTajna != tajna {
		t.Fatalf("dešifrovana tajna = %q, očekivano %q", svezi.TotpTajna, tajna)
	}
}

// brisanje TOTP-a (prazna tajna) upisuje NULL
func TestKorisniciTotpBrisanje(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	repo := NoviKorisniciRepo(db, testKljuc(t))

	k, _ := repo.Kreiraj(ctx, "pera", "hash", "radnik")
	_ = repo.SacuvajTotpTajnu(ctx, k.ID, "JBSWY3DPEHPK3PXP")
	if err := repo.SacuvajTotpTajnu(ctx, k.ID, ""); err != nil {
		t.Fatalf("brisanje: %v", err)
	}
	if sirova := sirovaTotpTajna(t, db, k.ID); sirova != "" {
		t.Fatalf("posle brisanja tajna nije prazna: %q", sirova)
	}
	svezi, _ := repo.DohvatiPoID(ctx, k.ID)
	if svezi.TotpTajna != "" {
		t.Fatalf("posle brisanja TotpTajna = %q, očekivano prazno", svezi.TotpTajna)
	}
}

// ZasifrujPostojeceTotp pretvara staru nešifrovanu tajnu u šifrovanu, a stvarnu
// vrednost ostavlja čitljivom kroz repo (fallback)
func TestZasifrujPostojeceTotp(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	kljuc := testKljuc(t)
	repo := NoviKorisniciRepo(db, kljuc)

	k, _ := repo.Kreiraj(ctx, "pera", "hash", "radnik")

	// simuliraj staru bazu: upiši čist tekst direktno (mimo repo-a)
	const tajna = "JBSWY3DPEHPK3PXP"
	if _, err := db.Exec(`UPDATE korisnici SET totp_tajna = ? WHERE id = ?`, tajna, k.ID); err != nil {
		t.Fatalf("upis plain-text tajne: %v", err)
	}

	// pre migracije repo i dalje vraća ispravnu tajnu (fallback na plain text)
	if svezi, _ := repo.DohvatiPoID(ctx, k.ID); svezi.TotpTajna != tajna {
		t.Fatalf("fallback nije vratio plain-text tajnu: %q", svezi.TotpTajna)
	}

	br, err := ZasifrujPostojeceTotp(ctx, db, kljuc)
	if err != nil {
		t.Fatalf("ZasifrujPostojeceTotp: %v", err)
	}
	if br != 1 {
		t.Fatalf("očekivano 1 šifrovan red, dobijeno %d", br)
	}

	// sada je u bazi šifrovano, ali kroz repo i dalje čitljivo
	if sirova := sirovaTotpTajna(t, db, k.ID); sirova == tajna {
		t.Fatal("posle migracije tajna je i dalje plain text")
	}
	if svezi, _ := repo.DohvatiPoID(ctx, k.ID); svezi.TotpTajna != tajna {
		t.Fatalf("posle migracije dešifrovana tajna = %q, očekivano %q", svezi.TotpTajna, tajna)
	}

	// idempotentno: drugo pokretanje ne menja ništa
	if br2, _ := ZasifrujPostojeceTotp(ctx, db, kljuc); br2 != 0 {
		t.Fatalf("ponovno pokretanje šifrovalo %d redova, očekivano 0", br2)
	}
}
