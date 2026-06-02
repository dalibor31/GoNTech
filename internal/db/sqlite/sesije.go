package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"ntech/internal/model"
)

type sqliteSesijeRepo struct{ db *sql.DB }

// NoviSesijeRepo kreira SQLite implementaciju SesijeRepository
func NoviSesijeRepo(db *sql.DB) *sqliteSesijeRepo {
	return &sqliteSesijeRepo{db: db}
}

func (r *sqliteSesijeRepo) Kreiraj(ctx context.Context, korisnikID int64, token string, istice time.Time, totpPotvrdjeno bool) error {
	potvrdjen := 1
	if !totpPotvrdjeno {
		potvrdjen = 0
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO sesije (korisnik_id, token, totp_potvrdjeno, datum_isteka) VALUES (?, ?, ?, ?)`,
		korisnikID, token, potvrdjen, istice)
	if err != nil {
		return fmt.Errorf("ntech: sesije.Kreiraj: %w", err)
	}
	return nil
}

func (r *sqliteSesijeRepo) DohvatiPoTokenu(ctx context.Context, token string) (*model.Sesija, error) {
	s := &model.Sesija{}
	var totpPotvrdjeno int
	var datumIsteka, datumKreiranja time.Time
	err := r.db.QueryRowContext(ctx,
		`SELECT id, korisnik_id, token, totp_potvrdjeno, datum_isteka, datum_kreiranja
		 FROM sesije WHERE token = ?`, token).
		Scan(&s.ID, &s.KorisnikID, &s.Token, &totpPotvrdjeno, &datumIsteka, &datumKreiranja)
	if err != nil {
		return nil, fmt.Errorf("ntech: sesije.DohvatiPoTokenu: %w", err)
	}
	s.TotpPotvrdjeno = totpPotvrdjeno == 1
	s.DatumIsteka = datumIsteka
	s.DatumKreiranja = datumKreiranja
	return s, nil
}

func (r *sqliteSesijeRepo) PotvrdiTotp(ctx context.Context, token string, novoIstice time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE sesije SET totp_potvrdjeno = 1, datum_isteka = ? WHERE token = ?`,
		novoIstice, token)
	if err != nil {
		return fmt.Errorf("ntech: sesije.PotvrdiTotp: %w", err)
	}
	return nil
}

func (r *sqliteSesijeRepo) Obrisi(ctx context.Context, token string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM sesije WHERE token = ?`, token)
	if err != nil {
		return fmt.Errorf("ntech: sesije.Obrisi: %w", err)
	}
	return nil
}

func (r *sqliteSesijeRepo) ObrisiIstekle(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM sesije WHERE datum_isteka < ?`, time.Now())
	if err != nil {
		return fmt.Errorf("ntech: sesije.ObrisiIstekle: %w", err)
	}
	return nil
}
