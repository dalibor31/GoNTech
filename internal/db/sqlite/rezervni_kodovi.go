package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"ntech/internal/auth"
)

type sqliteRezervniKodoviRepo struct{ db *sql.DB }

// NoviRezervniKodoviRepo kreira SQLite implementaciju RezervniKodoviRepository
func NoviRezervniKodoviRepo(db *sql.DB) *sqliteRezervniKodoviRepo {
	return &sqliteRezervniKodoviRepo{db: db}
}

// Zameni briše sve postojeće kodove korisnika i upisuje nove (regeneracija) u
// jednoj transakciji — stari kodovi se time poništavaju.
func (r *sqliteRezervniKodoviRepo) Zameni(ctx context.Context, korisnikID int64, hashevi []string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ntech: rezervni.Zameni: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM rezervni_kodovi WHERE korisnik_id = ?`, korisnikID); err != nil {
		return fmt.Errorf("ntech: rezervni.Zameni: %w", err)
	}
	for _, h := range hashevi {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO rezervni_kodovi (korisnik_id, kod_hash) VALUES (?, ?)`, korisnikID, h); err != nil {
			return fmt.Errorf("ntech: rezervni.Zameni: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("ntech: rezervni.Zameni: %w", err)
	}
	return nil
}

// Iskoristi proverava dati (već normalizovan) kod protiv neiskorišćenih kodova
// korisnika. Ako se poklopi, označava ga iskorišćenim i vraća true; inače false.
func (r *sqliteRezervniKodoviRepo) Iskoristi(ctx context.Context, korisnikID int64, kod string) (bool, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, kod_hash FROM rezervni_kodovi WHERE korisnik_id = ? AND iskoriscen = 0`, korisnikID)
	if err != nil {
		return false, fmt.Errorf("ntech: rezervni.Iskoristi: %w", err)
	}
	type red struct {
		id   int64
		hash string
	}
	var redovi []red
	for rows.Next() {
		var rd red
		if err := rows.Scan(&rd.id, &rd.hash); err != nil {
			rows.Close()
			return false, fmt.Errorf("ntech: rezervni.Iskoristi: %w", err)
		}
		redovi = append(redovi, rd)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("ntech: rezervni.Iskoristi: %w", err)
	}

	// bcrypt poređenje protiv svakog neiskorišćenog koda (retka operacija — fallback)
	for _, rd := range redovi {
		if auth.ProveriLozinku(rd.hash, kod) {
			if _, err := r.db.ExecContext(ctx,
				`UPDATE rezervni_kodovi SET iskoriscen = 1, datum_koriscenja = ? WHERE id = ?`,
				time.Now(), rd.id); err != nil {
				return false, fmt.Errorf("ntech: rezervni.Iskoristi: %w", err)
			}
			return true, nil
		}
	}
	return false, nil
}

func (r *sqliteRezervniKodoviRepo) BrojPreostalih(ctx context.Context, korisnikID int64) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM rezervni_kodovi WHERE korisnik_id = ? AND iskoriscen = 0`, korisnikID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("ntech: rezervni.BrojPreostalih: %w", err)
	}
	return n, nil
}

func (r *sqliteRezervniKodoviRepo) Obrisi(ctx context.Context, korisnikID int64) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM rezervni_kodovi WHERE korisnik_id = ?`, korisnikID); err != nil {
		return fmt.Errorf("ntech: rezervni.Obrisi: %w", err)
	}
	return nil
}
