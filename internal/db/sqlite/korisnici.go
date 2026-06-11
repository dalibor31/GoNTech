package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"ntech/internal/model"
)

type sqliteKorisniciRepo struct{ db *sql.DB }

// dodeliOpcijeKorisnika popunjava bool i opciona polja korisnika iz skeniranih
// NULL vrednosti — deljeno između skeniraiKorisnika (jedan red) i Lista (više redova)
func dodeliOpcijeKorisnika(k *model.Korisnik, aktivan, koristiLokalnuTemu int,
	lokalnaTema, lokalnaPozadina, lokalnaPozadinaOpacity, lokalnaPozadinaBlur,
	lokalnaPozadinaBlurPozadine, lokalnaPozadinaGlassOpacity sql.NullString,
	datumKreiranja time.Time) {
	k.Aktivan = aktivan == 1
	k.LokalnaTema = lokalnaTema.String
	k.KoristiLokalnuTemu = koristiLokalnuTemu == 1
	k.DatumKreiranja = datumKreiranja
	k.LokalnaPozadina = lokalnaPozadina.String
	k.LokalnaPozadinaOpacity = lokalnaPozadinaOpacity.String
	k.LokalnaPozadinaBlur = lokalnaPozadinaBlur.String
	k.LokalnaPozadinaBlurPozadine = lokalnaPozadinaBlurPozadine.String
	k.LokalnaPozadinaGlassOpacity = lokalnaPozadinaGlassOpacity.String
}

// skeniraiKorisnika čita jedan red iz baze i popunjava model.Korisnik
func skeniraiKorisnika(row interface{ Scan(...any) error }) (*model.Korisnik, error) {
	k := &model.Korisnik{}
	var aktivan, koristiLokalnuTemu int
	var lokalnaTema sql.NullString
	var lokalnaPozadina, lokalnaPozadinaOpacity, lokalnaPozadinaBlur, lokalnaPozadinaBlurPozadine, lokalnaPozadinaGlassOpacity sql.NullString
	var datumKreiranja time.Time
	if err := row.Scan(
		&k.ID, &k.KorisnickoIme, &k.LozinkaHash, &k.Uloga, &aktivan, &k.TotpTajna,
		&lokalnaTema, &koristiLokalnuTemu, &datumKreiranja,
		&lokalnaPozadina, &lokalnaPozadinaOpacity, &lokalnaPozadinaBlur,
		&lokalnaPozadinaBlurPozadine, &lokalnaPozadinaGlassOpacity,
	); err != nil {
		return nil, err
	}
	dodeliOpcijeKorisnika(k, aktivan, koristiLokalnuTemu, lokalnaTema,
		lokalnaPozadina, lokalnaPozadinaOpacity, lokalnaPozadinaBlur,
		lokalnaPozadinaBlurPozadine, lokalnaPozadinaGlassOpacity, datumKreiranja)
	return k, nil
}

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
	row := r.db.QueryRowContext(ctx,
		`SELECT id, korisnicko_ime, lozinka_hash, uloga, aktivan, COALESCE(totp_tajna, ''),
		        COALESCE(lokalna_tema, ''), koristi_lokalnu_temu, datum_kreiranja,
		        COALESCE(lokalna_pozadina, ''), COALESCE(lokalna_pozadina_opacity, '50'),
		        COALESCE(lokalna_pozadina_blur, '12'), COALESCE(lokalna_pozadina_blur_pozadine, '0'),
		        COALESCE(lokalna_pozadina_glass_opacity, '10')
		 FROM korisnici WHERE korisnicko_ime = ?`, korisnickoIme)
	k, err := skeniraiKorisnika(row)
	if err != nil {
		return nil, fmt.Errorf("ntech: korisnici.DohvatiPoImenu: %w", err)
	}
	return k, nil
}

func (r *sqliteKorisniciRepo) DohvatiPoID(ctx context.Context, id int64) (*model.Korisnik, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, korisnicko_ime, lozinka_hash, uloga, aktivan, COALESCE(totp_tajna, ''),
		        COALESCE(lokalna_tema, ''), koristi_lokalnu_temu, datum_kreiranja,
		        COALESCE(lokalna_pozadina, ''), COALESCE(lokalna_pozadina_opacity, '50'),
		        COALESCE(lokalna_pozadina_blur, '12'), COALESCE(lokalna_pozadina_blur_pozadine, '0'),
		        COALESCE(lokalna_pozadina_glass_opacity, '10')
		 FROM korisnici WHERE id = ?`, id)
	k, err := skeniraiKorisnika(row)
	if err != nil {
		return nil, fmt.Errorf("ntech: korisnici.DohvatiPoID: %w", err)
	}
	return k, nil
}

func (r *sqliteKorisniciRepo) Lista(ctx context.Context) ([]model.Korisnik, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, korisnicko_ime, uloga, aktivan, COALESCE(totp_tajna, ''),
		        COALESCE(lokalna_tema, ''), koristi_lokalnu_temu, datum_kreiranja,
		        COALESCE(lokalna_pozadina, ''), COALESCE(lokalna_pozadina_opacity, '50'),
		        COALESCE(lokalna_pozadina_blur, '12'), COALESCE(lokalna_pozadina_blur_pozadine, '0'),
		        COALESCE(lokalna_pozadina_glass_opacity, '10')
		 FROM korisnici ORDER BY datum_kreiranja ASC`)
	if err != nil {
		return nil, fmt.Errorf("ntech: korisnici.Lista: %w", err)
	}
	defer rows.Close()
	var lista []model.Korisnik
	for rows.Next() {
		var k model.Korisnik
		var aktivan, koristiLokalnuTemu int
		var lokalnaTema sql.NullString
		var lokalnaPozadina, lokalnaPozadinaOpacity, lokalnaPozadinaBlur, lokalnaPozadinaBlurPozadine, lokalnaPozadinaGlassOpacity sql.NullString
		var datumKreiranja time.Time
		if err := rows.Scan(&k.ID, &k.KorisnickoIme, &k.Uloga, &aktivan, &k.TotpTajna,
			&lokalnaTema, &koristiLokalnuTemu, &datumKreiranja,
			&lokalnaPozadina, &lokalnaPozadinaOpacity, &lokalnaPozadinaBlur, &lokalnaPozadinaBlurPozadine,
			&lokalnaPozadinaGlassOpacity); err != nil {
			return nil, fmt.Errorf("ntech: korisnici.Lista: %w", err)
		}
		dodeliOpcijeKorisnika(&k, aktivan, koristiLokalnuTemu, lokalnaTema,
			lokalnaPozadina, lokalnaPozadinaOpacity, lokalnaPozadinaBlur,
			lokalnaPozadinaBlurPozadine, lokalnaPozadinaGlassOpacity, datumKreiranja)
		lista = append(lista, k)
	}
	return lista, nil
}

func (r *sqliteKorisniciRepo) SacuvajLokalnuPozadinu(ctx context.Context, id int64, pozadina, opacity, blur, blurPozadine, glassOpacity string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE korisnici SET lokalna_pozadina = ?, lokalna_pozadina_opacity = ?, lokalna_pozadina_blur = ?, lokalna_pozadina_blur_pozadine = ?, lokalna_pozadina_glass_opacity = ? WHERE id = ?`,
		pozadina, opacity, blur, blurPozadine, glassOpacity, id)
	if err != nil {
		return fmt.Errorf("ntech: korisnici.SacuvajLokalnuPozadinu: %w", err)
	}
	return nil
}

func (r *sqliteKorisniciRepo) SacuvajLokalnuTemu(ctx context.Context, id int64, lokalnaTema string, koristi bool) error {
	koristiInt := 0
	if koristi {
		koristiInt = 1
	}
	var tema interface{}
	if lokalnaTema == "" {
		tema = nil
	} else {
		tema = lokalnaTema
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE korisnici SET lokalna_tema = ?, koristi_lokalnu_temu = ? WHERE id = ?`,
		tema, koristiInt, id)
	if err != nil {
		return fmt.Errorf("ntech: korisnici.SacuvajLokalnuTemu: %w", err)
	}
	return nil
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

func (r *sqliteKorisniciRepo) Obrisi(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM korisnici WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("ntech: korisnici.Obrisi: %w", err)
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
