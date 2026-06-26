package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"ntech/internal/auth"
	"ntech/internal/model"
)

// sqliteKorisniciRepo drži ključ za šifrovanje TOTP tajni (AES-256-GCM) jer se
// tajna šifruje pri upisu (SacuvajTotpTajnu) i dešifruje pri čitanju.
type sqliteKorisniciRepo struct {
	db    *sql.DB
	kljuc []byte
}

// korisnikOpcije drži NULL vrednosti skeniranih opcionalnih kolona korisnika;
// dodavanje novog polja zahteva izmenu samo ovog struct-a i relevantnih Scan poziva.
type korisnikOpcije struct {
	aktivan                     int
	koristiLokalnuTemu          int
	datumKreiranja              time.Time
	lokalnaTema                 sql.NullString
	lokalnaPozadina             sql.NullString
	lokalnaPozadinaOpacity      sql.NullString
	lokalnaPozadinaBlur         sql.NullString
	lokalnaPozadinaBlurPozadine sql.NullString
	lokalnaPozadinaGlassOpacity sql.NullString
	avatarPutanja               sql.NullString
	lokalnaAnimacija            sql.NullString
	lokalniHover                sql.NullString
	lokalnaBrzinaAnimacije      sql.NullString
	ime                         sql.NullString
	prezime                     sql.NullString
}

// dodeliOpcijeKorisnika prenosi vrednosti iz korisnikOpcije na model.Korisnik
func dodeliOpcijeKorisnika(k *model.Korisnik, o korisnikOpcije) {
	k.Aktivan = o.aktivan == 1
	k.KoristiLokalnuTemu = o.koristiLokalnuTemu == 1
	k.DatumKreiranja = o.datumKreiranja
	k.LokalnaTema = o.lokalnaTema.String
	k.LokalnaPozadina = o.lokalnaPozadina.String
	k.LokalnaPozadinaOpacity = o.lokalnaPozadinaOpacity.String
	k.LokalnaPozadinaBlur = o.lokalnaPozadinaBlur.String
	k.LokalnaPozadinaBlurPozadine = o.lokalnaPozadinaBlurPozadine.String
	k.LokalnaPozadinaGlassOpacity = o.lokalnaPozadinaGlassOpacity.String
	k.AvatarPutanja = o.avatarPutanja.String
	k.LokalnaAnimacija = o.lokalnaAnimacija.String
	k.LokalniHover = o.lokalniHover.String
	k.LokalnaBrzinaAnimacije = o.lokalnaBrzinaAnimacije.String
	k.Ime = o.ime.String
	k.Prezime = o.prezime.String
}

// skeniraiKorisnika čita jedan red iz baze i popunjava model.Korisnik
func skeniraiKorisnika(row interface{ Scan(...any) error }) (*model.Korisnik, error) {
	k := &model.Korisnik{}
	var o korisnikOpcije
	if err := row.Scan(
		&k.ID, &k.KorisnickoIme, &k.LozinkaHash, &k.Uloga, &o.aktivan, &k.TotpTajna,
		&o.lokalnaTema, &o.koristiLokalnuTemu, &o.datumKreiranja,
		&o.lokalnaPozadina, &o.lokalnaPozadinaOpacity, &o.lokalnaPozadinaBlur,
		&o.lokalnaPozadinaBlurPozadine, &o.lokalnaPozadinaGlassOpacity, &o.avatarPutanja,
		&o.lokalnaAnimacija, &o.lokalniHover, &o.lokalnaBrzinaAnimacije,
		&o.ime, &o.prezime,
	); err != nil {
		return nil, err
	}
	dodeliOpcijeKorisnika(k, o)
	return k, nil
}

// NoviKorisniciRepo kreira SQLite implementaciju KorisniciRepository.
// kljuc je 32-bajtni ključ za šifrovanje TOTP tajni u mirovanju.
func NoviKorisniciRepo(db *sql.DB, kljuc []byte) *sqliteKorisniciRepo {
	return &sqliteKorisniciRepo{db: db, kljuc: kljuc}
}

// desifrujTotpTajnu pretvara šifrovanu TOTP tajnu (kako stoji u bazi) u čist
// tekst u memoriji. Tolerantno je na stare nešifrovane tajne: ako dešifrovanje
// ne uspe (greška GCM provere), vrednost se ostavlja kakva jeste — to je plain
// text iz verzija pre uvođenja enkripcije.
func (r *sqliteKorisniciRepo) desifrujTotpTajnu(k *model.Korisnik) {
	if k.TotpTajna == "" {
		return
	}
	if cisto, err := auth.Desifruj(k.TotpTajna, r.kljuc); err == nil {
		k.TotpTajna = cisto
	}
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
		        COALESCE(lokalna_pozadina_glass_opacity, '10'), COALESCE(avatar_putanja, ''),
		        COALESCE(lokalna_animacija, ''), COALESCE(lokalni_hover, ''),
		        COALESCE(lokalna_brzina_animacije, ''),
		        COALESCE(ime, ''), COALESCE(prezime, '')
		 FROM korisnici WHERE korisnicko_ime = ?`, korisnickoIme)
	k, err := skeniraiKorisnika(row)
	if err != nil {
		return nil, fmt.Errorf("ntech: korisnici.DohvatiPoImenu: %w", err)
	}
	r.desifrujTotpTajnu(k)
	return k, nil
}

func (r *sqliteKorisniciRepo) DohvatiPoID(ctx context.Context, id int64) (*model.Korisnik, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, korisnicko_ime, lozinka_hash, uloga, aktivan, COALESCE(totp_tajna, ''),
		        COALESCE(lokalna_tema, ''), koristi_lokalnu_temu, datum_kreiranja,
		        COALESCE(lokalna_pozadina, ''), COALESCE(lokalna_pozadina_opacity, '50'),
		        COALESCE(lokalna_pozadina_blur, '12'), COALESCE(lokalna_pozadina_blur_pozadine, '0'),
		        COALESCE(lokalna_pozadina_glass_opacity, '10'), COALESCE(avatar_putanja, ''),
		        COALESCE(lokalna_animacija, ''), COALESCE(lokalni_hover, ''),
		        COALESCE(lokalna_brzina_animacije, ''),
		        COALESCE(ime, ''), COALESCE(prezime, '')
		 FROM korisnici WHERE id = ?`, id)
	k, err := skeniraiKorisnika(row)
	if err != nil {
		return nil, fmt.Errorf("ntech: korisnici.DohvatiPoID: %w", err)
	}
	r.desifrujTotpTajnu(k)
	return k, nil
}

func (r *sqliteKorisniciRepo) Lista(ctx context.Context) ([]model.Korisnik, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, korisnicko_ime, uloga, aktivan, COALESCE(totp_tajna, ''),
		        COALESCE(lokalna_tema, ''), koristi_lokalnu_temu, datum_kreiranja,
		        COALESCE(lokalna_pozadina, ''), COALESCE(lokalna_pozadina_opacity, '50'),
		        COALESCE(lokalna_pozadina_blur, '12'), COALESCE(lokalna_pozadina_blur_pozadine, '0'),
		        COALESCE(lokalna_pozadina_glass_opacity, '10'), COALESCE(avatar_putanja, ''),
		        COALESCE(lokalna_animacija, ''), COALESCE(lokalni_hover, ''),
		        COALESCE(lokalna_brzina_animacije, ''),
		        COALESCE(ime, ''), COALESCE(prezime, '')
		 FROM korisnici ORDER BY datum_kreiranja ASC`)
	if err != nil {
		return nil, fmt.Errorf("ntech: korisnici.Lista: %w", err)
	}
	defer rows.Close()
	var lista []model.Korisnik
	for rows.Next() {
		var k model.Korisnik
		var o korisnikOpcije
		if err := rows.Scan(
			&k.ID, &k.KorisnickoIme, &k.Uloga, &o.aktivan, &k.TotpTajna,
			&o.lokalnaTema, &o.koristiLokalnuTemu, &o.datumKreiranja,
			&o.lokalnaPozadina, &o.lokalnaPozadinaOpacity, &o.lokalnaPozadinaBlur,
			&o.lokalnaPozadinaBlurPozadine, &o.lokalnaPozadinaGlassOpacity, &o.avatarPutanja,
			&o.lokalnaAnimacija, &o.lokalniHover, &o.lokalnaBrzinaAnimacije,
			&o.ime, &o.prezime,
		); err != nil {
			return nil, fmt.Errorf("ntech: korisnici.Lista: %w", err)
		}
		dodeliOpcijeKorisnika(&k, o)
		r.desifrujTotpTajnu(&k)
		lista = append(lista, k)
	}
	return lista, nil
}

func (r *sqliteKorisniciRepo) AzurirajImePrezime(ctx context.Context, id int64, ime, prezime string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE korisnici SET ime = ?, prezime = ? WHERE id = ?`, ime, prezime, id)
	if err != nil {
		return fmt.Errorf("ntech: korisnici.AzurirajImePrezime: %w", err)
	}
	return nil
}

func (r *sqliteKorisniciRepo) SacuvajAvatar(ctx context.Context, id int64, putanja string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE korisnici SET avatar_putanja = ? WHERE id = ?`, putanja, id)
	if err != nil {
		return fmt.Errorf("ntech: korisnici.SacuvajAvatar: %w", err)
	}
	return nil
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

func (r *sqliteKorisniciRepo) SacuvajLokalnuAnimaciju(ctx context.Context, id int64, animacija string) error {
	var val any
	if animacija != "" {
		val = animacija
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE korisnici SET lokalna_animacija = ? WHERE id = ?`, val, id)
	if err != nil {
		return fmt.Errorf("ntech: korisnici.SacuvajLokalnuAnimaciju: %w", err)
	}
	return nil
}

func (r *sqliteKorisniciRepo) SacuvajLokalniHover(ctx context.Context, id int64, hover string) error {
	var val any
	if hover != "" {
		val = hover
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE korisnici SET lokalni_hover = ? WHERE id = ?`, val, id)
	if err != nil {
		return fmt.Errorf("ntech: korisnici.SacuvajLokalniHover: %w", err)
	}
	return nil
}

func (r *sqliteKorisniciRepo) SacuvajLokalnuBrzinuAnimacije(ctx context.Context, id int64, brzina string) error {
	var val any
	if brzina != "" {
		val = brzina
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE korisnici SET lokalna_brzina_animacije = ? WHERE id = ?`, val, id)
	if err != nil {
		return fmt.Errorf("ntech: korisnici.SacuvajLokalnuBrzinuAnimacije: %w", err)
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
		// tajna se čuva šifrovana (AES-256-GCM) — nikad kao čist tekst
		sifrovana, errSifra := auth.Sifruj(tajna, r.kljuc)
		if errSifra != nil {
			return fmt.Errorf("ntech: korisnici.SacuvajTotpTajnu: %w", errSifra)
		}
		_, err = r.db.ExecContext(ctx, `UPDATE korisnici SET totp_tajna = ? WHERE id = ?`, sifrovana, id)
	}
	if err != nil {
		return fmt.Errorf("ntech: korisnici.SacuvajTotpTajnu: %w", err)
	}
	return nil
}

// ZasifrujPostojeceTotp jednokratno šifruje sve TOTP tajne koje su u bazi ostale
// kao čist tekst (iz verzija pre uvođenja enkripcije). Idempotentno je: tajne koje
// se već dešifruju datim ključem preskače. Vraća broj ažuriranih redova. Poziva se
// pri pokretanju programa, posle migracija.
func ZasifrujPostojeceTotp(ctx context.Context, db *sql.DB, kljuc []byte) (int, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, totp_tajna FROM korisnici WHERE totp_tajna IS NOT NULL AND totp_tajna != ''`)
	if err != nil {
		return 0, fmt.Errorf("ntech: korisnici.ZasifrujPostojeceTotp: %w", err)
	}
	// prvo skupljamo redove, pa tek onda upisujemo — da ne čitamo i pišemo
	// istovremeno preko iste konekcije
	type red struct {
		id    int64
		tajna string
	}
	var zaSifrovanje []red
	for rows.Next() {
		var rd red
		if err := rows.Scan(&rd.id, &rd.tajna); err != nil {
			rows.Close()
			return 0, fmt.Errorf("ntech: korisnici.ZasifrujPostojeceTotp: %w", err)
		}
		// ako se već dešifruje, znači da je šifrovana — preskoči je
		if _, err := auth.Desifruj(rd.tajna, kljuc); err == nil {
			continue
		}
		zaSifrovanje = append(zaSifrovanje, rd)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, fmt.Errorf("ntech: korisnici.ZasifrujPostojeceTotp: %w", err)
	}
	rows.Close()

	br := 0
	for _, rd := range zaSifrovanje {
		sifrovana, err := auth.Sifruj(rd.tajna, kljuc)
		if err != nil {
			return br, fmt.Errorf("ntech: korisnici.ZasifrujPostojeceTotp: %w", err)
		}
		if _, err := db.ExecContext(ctx, `UPDATE korisnici SET totp_tajna = ? WHERE id = ?`, sifrovana, rd.id); err != nil {
			return br, fmt.Errorf("ntech: korisnici.ZasifrujPostojeceTotp: %w", err)
		}
		br++
	}
	return br, nil
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
