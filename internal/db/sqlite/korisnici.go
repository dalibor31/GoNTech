package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"ntech/internal/model"
)

type sqliteKorisniciRepo struct{ db *sql.DB }

// NoviKorisniciRepo kreira SQLite implementaciju KorisniciRepository
func NoviKorisniciRepo(db *sql.DB) *sqliteKorisniciRepo {
	return &sqliteKorisniciRepo{db: db}
}

func (r *sqliteKorisniciRepo) Kreiraj(ctx context.Context, korisnickoIme, lozinkaHash, uloga string) (*model.Korisnik, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO korisnici (korisnicko_ime, lozinka_hash, uloga) VALUES (?, ?, ?)`,
		korisnickoIme, lozinkaHash, uloga)
	if err != nil {
		return nil, fmt.Errorf("ntech: korisnici.Kreiraj: %w", err)
	}
	id, _ := res.LastInsertId()
	return r.DohvatiPoID(ctx, id)
}

func (r *sqliteKorisniciRepo) DohvatiPoImenu(ctx context.Context, korisnickoIme string) (*model.Korisnik, error) {
	k := &model.Korisnik{}
	var aktivan int
	var totpTajna sql.NullString
	var datumKreiranja time.Time
	err := r.db.QueryRowContext(ctx,
		`SELECT id, korisnicko_ime, lozinka_hash, uloga, aktivan, COALESCE(totp_tajna, ''), datum_kreiranja
		 FROM korisnici WHERE korisnicko_ime = ?`, korisnickoIme).
		Scan(&k.ID, &k.KorisnickoIme, &k.LozinkaHash, &k.Uloga, &aktivan, &totpTajna, &datumKreiranja)
	if err != nil {
		return nil, fmt.Errorf("ntech: korisnici.DohvatiPoImenu: %w", err)
	}
	k.Aktivan = aktivan == 1
	k.TotpTajna = totpTajna.String
	k.DatumKreiranja = datumKreiranja
	return k, nil
}

func (r *sqliteKorisniciRepo) DohvatiPoID(ctx context.Context, id int64) (*model.Korisnik, error) {
	k := &model.Korisnik{}
	var aktivan int
	var datumKreiranja time.Time
	err := r.db.QueryRowContext(ctx,
		`SELECT id, korisnicko_ime, lozinka_hash, uloga, aktivan, COALESCE(totp_tajna, ''), datum_kreiranja
		 FROM korisnici WHERE id = ?`, id).
		Scan(&k.ID, &k.KorisnickoIme, &k.LozinkaHash, &k.Uloga, &aktivan, &k.TotpTajna, &datumKreiranja)
	if err != nil {
		return nil, fmt.Errorf("ntech: korisnici.DohvatiPoID: %w", err)
	}
	k.Aktivan = aktivan == 1
	k.DatumKreiranja = datumKreiranja
	return k, nil
}

func (r *sqliteKorisniciRepo) Lista(ctx context.Context) ([]model.Korisnik, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, korisnicko_ime, lozinka_hash, uloga, aktivan, COALESCE(totp_tajna, ''), datum_kreiranja
		 FROM korisnici ORDER BY datum_kreiranja ASC`)
	if err != nil {
		return nil, fmt.Errorf("ntech: korisnici.Lista: %w", err)
	}
	defer rows.Close()
	var lista []model.Korisnik
	for rows.Next() {
		var k model.Korisnik
		var aktivan int
		var datumKreiranja time.Time
		if err := rows.Scan(&k.ID, &k.KorisnickoIme, &k.LozinkaHash, &k.Uloga, &aktivan, &k.TotpTajna, &datumKreiranja); err != nil {
			return nil, fmt.Errorf("ntech: korisnici.Lista: %w", err)
		}
		k.Aktivan = aktivan == 1
		k.DatumKreiranja = datumKreiranja
		lista = append(lista, k)
	}
	return lista, nil
}

func (r *sqliteKorisniciRepo) AzurirajUlogu(ctx context.Context, id int64, uloga string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE korisnici SET uloga = ? WHERE id = ?`, uloga, id)
	if err != nil {
		return fmt.Errorf("ntech: korisnici.AzurirajUlogu: %w", err)
	}
	return nil
}

func (r *sqliteKorisniciRepo) AzurirajAktivan(ctx context.Context, id int64, aktivan bool) error {
	val := 0
	if aktivan {
		val = 1
	}
	_, err := r.db.ExecContext(ctx, `UPDATE korisnici SET aktivan = ? WHERE id = ?`, val, id)
	if err != nil {
		return fmt.Errorf("ntech: korisnici.AzurirajAktivan: %w", err)
	}
	return nil
}

func (r *sqliteKorisniciRepo) PromeniLozinku(ctx context.Context, id int64, hash string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE korisnici SET lozinka_hash = ? WHERE id = ?`, hash, id)
	if err != nil {
		return fmt.Errorf("ntech: korisnici.PromeniLozinku: %w", err)
	}
	return nil
}

func (r *sqliteKorisniciRepo) SacuvajTotpTajnu(ctx context.Context, id int64, tajna string) error {
	var err error
	if tajna == "" {
		_, err = r.db.ExecContext(ctx, `UPDATE korisnici SET totp_tajna = NULL WHERE id = ?`, id)
	} else {
		_, err = r.db.ExecContext(ctx, `UPDATE korisnici SET totp_tajna = ? WHERE id = ?`, tajna, id)
	}
	if err != nil {
		return fmt.Errorf("ntech: korisnici.SacuvajTotpTajnu: %w", err)
	}
	return nil
}

func (r *sqliteKorisniciRepo) PostojiIjedan(ctx context.Context) (bool, error) {
	var broj int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM korisnici`).Scan(&broj)
	if err != nil {
		return false, fmt.Errorf("ntech: korisnici.PostojiIjedan: %w", err)
	}
	return broj > 0, nil
}
