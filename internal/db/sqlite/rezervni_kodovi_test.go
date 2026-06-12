package sqlite

import (
	"context"
	"testing"

	"ntech/internal/auth"
)

// hashirajKodove generiše n kodova i vraća (čisti kodovi, bcrypt hešovi)
func hashirajKodove(t *testing.T, n int) ([]string, []string) {
	t.Helper()
	kodovi, err := auth.GenerisiRezervneKodove(n)
	if err != nil {
		t.Fatalf("GenerisiRezervneKodove: %v", err)
	}
	var hashevi []string
	for _, kod := range kodovi {
		h, err := auth.HashujLozinku(kod)
		if err != nil {
			t.Fatalf("hash: %v", err)
		}
		hashevi = append(hashevi, h)
	}
	return kodovi, hashevi
}

func TestRezervniKodoviTok(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	korisnici := NoviKorisniciRepo(db, testKljuc(t))
	repo := NoviRezervniKodoviRepo(db)

	k, err := korisnici.Kreiraj(ctx, "pera", "hash", "radnik")
	if err != nil {
		t.Fatalf("Kreiraj: %v", err)
	}

	kodovi, hashevi := hashirajKodove(t, 5)
	if err := repo.Zameni(ctx, k.ID, hashevi); err != nil {
		t.Fatalf("Zameni: %v", err)
	}
	if n, _ := repo.BrojPreostalih(ctx, k.ID); n != 5 {
		t.Fatalf("BrojPreostalih = %d, očekivano 5", n)
	}

	// upotreba prvog koda
	if ok, err := repo.Iskoristi(ctx, k.ID, kodovi[0]); err != nil || !ok {
		t.Fatalf("Iskoristi(prvi): ok=%v err=%v", ok, err)
	}
	// jednokratni — isti kod drugi put ne prolazi
	if ok, _ := repo.Iskoristi(ctx, k.ID, kodovi[0]); ok {
		t.Fatal("isti kod NE sme da prođe drugi put")
	}
	if n, _ := repo.BrojPreostalih(ctx, k.ID); n != 4 {
		t.Fatalf("posle upotrebe BrojPreostalih = %d, očekivano 4", n)
	}

	// nepostojeći kod ne prolazi
	if ok, _ := repo.Iskoristi(ctx, k.ID, "XXXX-XXXX"); ok {
		t.Fatal("nepostojeći kod ne sme da prođe")
	}

	// Zameni poništava sve stare i postavlja nove
	noviKodovi, noviHash := hashirajKodove(t, 3)
	if err := repo.Zameni(ctx, k.ID, noviHash); err != nil {
		t.Fatalf("Zameni (regeneracija): %v", err)
	}
	if n, _ := repo.BrojPreostalih(ctx, k.ID); n != 3 {
		t.Fatalf("posle regeneracije BrojPreostalih = %d, očekivano 3", n)
	}
	if ok, _ := repo.Iskoristi(ctx, k.ID, kodovi[1]); ok {
		t.Fatal("stari kod posle regeneracije ne sme da prođe")
	}
	if ok, err := repo.Iskoristi(ctx, k.ID, noviKodovi[0]); err != nil || !ok {
		t.Fatalf("nov kod treba da prođe: ok=%v err=%v", ok, err)
	}

	// Obrisi (deaktivacija 2FA)
	if err := repo.Obrisi(ctx, k.ID); err != nil {
		t.Fatalf("Obrisi: %v", err)
	}
	if n, _ := repo.BrojPreostalih(ctx, k.ID); n != 0 {
		t.Fatalf("posle Obrisi BrojPreostalih = %d, očekivano 0", n)
	}
}
