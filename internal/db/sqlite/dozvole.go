package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type sqliteDozvoleRepo struct {
	db        *sql.DB
	defaultFn func(uloga, akcija string) bool // podrazumevane vrednosti iz middleware/dozvole.go
	sveAkcije []string
}

// NoviDozvoleRepo kreira SQLite implementaciju DozvoleRepository
func NoviDozvoleRepo(db *sql.DB, defaultFn func(uloga, akcija string) bool, sveAkcije []string) *sqliteDozvoleRepo {
	return &sqliteDozvoleRepo{db: db, defaultFn: defaultFn, sveAkcije: sveAkcije}
}

// InicijalizujDozvole popunjava tabelu podrazumevanim vrednostima ako je prazna
func InicijalizujDozvole(ctx context.Context, db *sql.DB, defaultFn func(uloga, akcija string) bool, sveAkcije []string) error {
	var br int
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM dozvole`).Scan(&br)
	if br > 0 {
		return nil
	}
	return popuniPodrazumevano(ctx, db, defaultFn, sveAkcije)
}

// OcistiSirociceDoz briše iz tabele dozvola redove čija akcija više ne postoji
// u sveAkcije (npr. nakon uklanjanja zastarele dozvole iz koda). Vraća broj obrisanih.
func OcistiSirociceDoz(ctx context.Context, db *sql.DB, sveAkcije []string) (int64, error) {
	if len(sveAkcije) == 0 {
		// sigurnosna brana — bez liste bismo obrisali sve, što ne želimo
		return 0, nil
	}
	placeholders := strings.Repeat("?,", len(sveAkcije))
	placeholders = strings.TrimSuffix(placeholders, ",")
	args := make([]any, len(sveAkcije))
	for i, a := range sveAkcije {
		args[i] = a
	}
	rezultat, err := db.ExecContext(ctx,
		`DELETE FROM dozvole WHERE akcija NOT IN (`+placeholders+`)`, args...)
	if err != nil {
		return 0, fmt.Errorf("ntech: dozvole.OcistiSirocice: %w", err)
	}
	br, _ := rezultat.RowsAffected()
	return br, nil
}

// popuniPodrazumevano upisuje sve podrazumevane dozvole u bazu
func popuniPodrazumevano(ctx context.Context, db *sql.DB, defaultFn func(uloga, akcija string) bool, sveAkcije []string) error {
	for _, uloga := range []string{"radnik", "admin", "superadmin"} {
		for _, akcija := range sveAkcije {
			dozvoljeno := 0
			if defaultFn(uloga, akcija) {
				dozvoljeno = 1
			}
			if _, err := db.ExecContext(ctx,
				`INSERT OR IGNORE INTO dozvole (uloga, akcija, dozvoljeno) VALUES (?, ?, ?)`,
				uloga, akcija, dozvoljeno); err != nil {
				return fmt.Errorf("ntech: dozvole.Inicijalizuj: %w", err)
			}
		}
	}
	return nil
}

func (r *sqliteDozvoleRepo) ImaDozvolu(ctx context.Context, uloga, akcija string) bool {
	var dozvoljeno int
	err := r.db.QueryRowContext(ctx,
		`SELECT dozvoljeno FROM dozvole WHERE uloga = ? AND akcija = ?`, uloga, akcija).Scan(&dozvoljeno)
	if err != nil {
		// fallback na podrazumevano ako red nije pronađen
		return r.defaultFn(uloga, akcija)
	}
	return dozvoljeno == 1
}

func (r *sqliteDozvoleRepo) SveDozvole(ctx context.Context, uloga string) map[string]bool {
	rows, err := r.db.QueryContext(ctx,
		`SELECT akcija, dozvoljeno FROM dozvole WHERE uloga = ?`, uloga)
	if err != nil {
		// fallback na podrazumevano
		m := make(map[string]bool, len(r.sveAkcije))
		for _, a := range r.sveAkcije {
			m[a] = r.defaultFn(uloga, a)
		}
		return m
	}
	defer rows.Close()

	m := make(map[string]bool, len(r.sveAkcije))
	for rows.Next() {
		var akcija string
		var dozvoljeno int
		if err := rows.Scan(&akcija, &dozvoljeno); err == nil {
			m[akcija] = dozvoljeno == 1
		}
	}
	// popuni eventualno nedostajuće akcije podrazumevanim vrednostima
	for _, a := range r.sveAkcije {
		if _, ok := m[a]; !ok {
			m[a] = r.defaultFn(uloga, a)
		}
	}
	return m
}

func (r *sqliteDozvoleRepo) Sacuvaj(ctx context.Context, uloga, akcija string, dozvoljeno bool) error {
	d := 0
	if dozvoljeno {
		d = 1
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO dozvole (uloga, akcija, dozvoljeno) VALUES (?, ?, ?)
		 ON CONFLICT (uloga, akcija) DO UPDATE SET dozvoljeno = excluded.dozvoljeno`,
		uloga, akcija, d)
	if err != nil {
		return fmt.Errorf("ntech: dozvole.Sacuvaj: %w", err)
	}
	return nil
}

func (r *sqliteDozvoleRepo) Reset(ctx context.Context) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM dozvole`); err != nil {
		return fmt.Errorf("ntech: dozvole.Reset: %w", err)
	}
	return popuniPodrazumevano(ctx, r.db, r.defaultFn, r.sveAkcije)
}
