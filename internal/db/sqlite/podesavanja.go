package sqlite

import (
	"context"
	"database/sql"
	"fmt"
)

// DohvatiPodesavanje čita jednu vrednost iz tabele podešavanja
func DohvatiPodesavanje(ctx context.Context, db *sql.DB, kljuc string) (string, error) {
	var vrednost string
	err := db.QueryRowContext(ctx, "SELECT vrednost FROM podesavanja WHERE kljuc = ?", kljuc).Scan(&vrednost)
	if err != nil {
		return "", fmt.Errorf("ntech: DohvatiPodesavanje: %s: %w", kljuc, err)
	}
	return vrednost, nil
}

// SacuvajPodesavanje upisuje ili menja vrednost u tabeli podešavanja
func SacuvajPodesavanje(ctx context.Context, db *sql.DB, kljuc, vrednost string) error {
	_, err := db.ExecContext(ctx,
		"INSERT INTO podesavanja (kljuc, vrednost) VALUES (?, ?) ON CONFLICT(kljuc) DO UPDATE SET vrednost = excluded.vrednost",
		kljuc, vrednost,
	)
	if err != nil {
		return fmt.Errorf("ntech: SacuvajPodesavanje: %s: %w", kljuc, err)
	}
	return nil
}

// DohvatiSvaPodesavanja čita sva podešavanja i vraća ih kao mapu
func DohvatiSvaPodesavanja(ctx context.Context, db *sql.DB) (map[string]string, error) {
	redovi, err := db.QueryContext(ctx, "SELECT kljuc, vrednost FROM podesavanja")
	if err != nil {
		return nil, fmt.Errorf("ntech: DohvatiSvaPodesavanja: %w", err)
	}
	defer redovi.Close()

	rezultat := make(map[string]string)
	for redovi.Next() {
		var kljuc, vrednost string
		if err := redovi.Scan(&kljuc, &vrednost); err != nil {
			return nil, fmt.Errorf("ntech: DohvatiSvaPodesavanja: scan: %w", err)
		}
		rezultat[kljuc] = vrednost
	}
	return rezultat, nil
}
