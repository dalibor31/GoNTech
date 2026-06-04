package sqlite

import (
	"database/sql"
	"fmt"
	"io/fs"
	"path"
	"sort"

	_ "modernc.org/sqlite"
)

// OtvoriDB otvara konekciju ka SQLite bazi i primenjuje performance PRAGMA podešavanja
func OtvoriDB(putanja string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", putanja)
	if err != nil {
		return nil, fmt.Errorf("ntech: OtvoriDB: %w", err)
	}

	pragme := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA cache_size=10000",
		"PRAGMA foreign_keys=ON",
	}
	for _, p := range pragme {
		if _, err := db.Exec(p); err != nil {
			return nil, fmt.Errorf("ntech: OtvoriDB: %s: %w", p, err)
		}
	}

	return db, nil
}

// PokreniMigracije izvršava sve SQL fajlove iz fs.FS koji još nisu izvršeni
func PokreniMigracije(db *sql.DB, fsys fs.FS) error {
	// kreiramo tabelu za praćenje migracija ako ne postoji
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS migracije (
			naziv               TEXT PRIMARY KEY,
			datum_izvrsavanja   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("ntech: PokreniMigracije: kreiranje tabele: %w", err)
	}

	// čitamo sve .sql fajlove iz foldera
	unosi, err := fs.ReadDir(fsys, "migrations")
	if err != nil {
		return fmt.Errorf("ntech: PokreniMigracije: čitanje foldera: %w", err)
	}

	// sortiramo po imenu da bi se izvršavali po redu (001, 002, ...)
	sort.Slice(unosi, func(i, j int) bool {
		return unosi[i].Name() < unosi[j].Name()
	})

	for _, unos := range unosi {
		if path.Ext(unos.Name()) != ".sql" {
			continue
		}

		naziv := unos.Name()

		// proveravamo da li je migracija već izvršena
		var broj int
		err := db.QueryRow("SELECT COUNT(*) FROM migracije WHERE naziv = ?", naziv).Scan(&broj)
		if err != nil {
			return fmt.Errorf("ntech: PokreniMigracije: provera %s: %w", naziv, err)
		}
		if broj > 0 {
			continue
		}

		// čitamo sadržaj SQL fajla
		putanja := "migrations/" + naziv
		sadrzaj, err := fs.ReadFile(fsys, putanja)
		if err != nil {
			return fmt.Errorf("ntech: PokreniMigracije: čitanje %s: %w", naziv, err)
		}

		// izvršavamo SQL
		if _, err := db.Exec(string(sadrzaj)); err != nil {
			return fmt.Errorf("ntech: PokreniMigracije: izvršavanje %s: %w", naziv, err)
		}

		// upisujemo u tabelu migracija da je izvršena
		if _, err := db.Exec("INSERT INTO migracije (naziv) VALUES (?)", naziv); err != nil {
			return fmt.Errorf("ntech: PokreniMigracije: upis %s: %w", naziv, err)
		}
	}

	return nil
}
