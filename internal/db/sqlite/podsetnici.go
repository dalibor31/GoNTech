package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"ntech/internal/db"
	"ntech/internal/model"
)

// PodsetnikRepo je SQLite implementacija PodsetnikRepository interfejsa
type PodsetnikRepo struct {
	db *sql.DB
}

// NoviPodsetnikRepo kreira novi PodsetnikRepo
func NoviPodsetnikRepo(db *sql.DB) *PodsetnikRepo {
	return &PodsetnikRepo{db: db}
}

// Lista vraća listu podsetnika sa opcionim filterom
func (r *PodsetnikRepo) Lista(ctx context.Context, filter db.PodsetnikFilter) ([]model.Podsetnik, error) {
	upit := `
		SELECT id, naslov, napomena, datum_podsecanja, zavrseno, tip, datum_unosa, korisnik_id
		FROM podsetnici
		WHERE 1=1`

	var args []any

	if filter.SamoAktivni {
		upit += " AND zavrseno = 0"
	}

	if filter.KorisnikID != nil {
		upit += " AND korisnik_id = ?"
		args = append(args, *filter.KorisnikID)
	}

	upit += " ORDER BY datum_podsecanja ASC"

	redovi, err := r.db.QueryContext(ctx, upit, args...)
	if err != nil {
		return nil, fmt.Errorf("ntech: PodsetnikRepo.Lista: %w", err)
	}
	defer redovi.Close()

	var rezultat []model.Podsetnik
	for redovi.Next() {
		var p model.Podsetnik
		var napomena sql.NullString
		var korisnikID sql.NullInt64
		var zavrseno int
		err := redovi.Scan(
			&p.ID, &p.Naslov, &napomena, &p.DatumPodsecanja, &zavrseno, &p.Tip, &p.DatumUnosa, &korisnikID,
		)
		if err != nil {
			return nil, fmt.Errorf("ntech: PodsetnikRepo.Lista: scan: %w", err)
		}
		p.Napomena = napomena.String
		p.Zavrseno = zavrseno == 1
		if korisnikID.Valid {
			p.KorisnikID = &korisnikID.Int64
		}
		rezultat = append(rezultat, p)
	}

	return rezultat, nil
}

// DohvatiID vraća jedan podsetnik po ID-u
func (r *PodsetnikRepo) DohvatiID(ctx context.Context, id int64) (*model.Podsetnik, error) {
	var p model.Podsetnik
	var napomena sql.NullString
	var korisnikID sql.NullInt64
	var zavrseno int

	err := r.db.QueryRowContext(ctx, `
		SELECT id, naslov, napomena, datum_podsecanja, zavrseno, tip, datum_unosa, korisnik_id
		FROM podsetnici WHERE id = ?`, id).Scan(
		&p.ID, &p.Naslov, &napomena, &p.DatumPodsecanja, &zavrseno, &p.Tip, &p.DatumUnosa, &korisnikID,
	)
	if err != nil {
		return nil, fmt.Errorf("ntech: PodsetnikRepo.DohvatiID: %w", err)
	}

	p.Napomena = napomena.String
	p.Zavrseno = zavrseno == 1
	if korisnikID.Valid {
		p.KorisnikID = &korisnikID.Int64
	}

	return &p, nil
}

// Kreiraj dodaje novi podsetnik u bazu
func (r *PodsetnikRepo) Kreiraj(ctx context.Context, p *model.Podsetnik) (int64, error) {
	var korisnikID interface{}
	if p.KorisnikID != nil {
		korisnikID = *p.KorisnikID
	}

	rezultat, err := r.db.ExecContext(ctx, `
		INSERT INTO podsetnici (naslov, napomena, datum_podsecanja, tip, korisnik_id)
		VALUES (?, ?, ?, ?, ?)`,
		p.Naslov, nullString(p.Napomena), p.DatumPodsecanja, p.Tip, korisnikID,
	)
	if err != nil {
		return 0, fmt.Errorf("ntech: PodsetnikRepo.Kreiraj: %w", err)
	}

	id, err := rezultat.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("ntech: PodsetnikRepo.Kreiraj: last insert id: %w", err)
	}

	return id, nil
}

// Izmeni ažurira postojeći podsetnik
func (r *PodsetnikRepo) Izmeni(ctx context.Context, p *model.Podsetnik) error {
	var korisnikID interface{}
	if p.KorisnikID != nil {
		korisnikID = *p.KorisnikID
	}

	_, err := r.db.ExecContext(ctx, `
		UPDATE podsetnici SET
			naslov = ?, napomena = ?, datum_podsecanja = ?, tip = ?, korisnik_id = ?
		WHERE id = ?`,
		p.Naslov, nullString(p.Napomena), p.DatumPodsecanja, p.Tip, korisnikID, p.ID,
	)
	if err != nil {
		return fmt.Errorf("ntech: PodsetnikRepo.Izmeni: %w", err)
	}

	return nil
}

// OznaciZavrsenim postavlja ili uklanja oznaku završenosti
func (r *PodsetnikRepo) OznaciZavrsenim(ctx context.Context, id int64, zavrseno bool) error {
	val := 0
	if zavrseno {
		val = 1
	}
	_, err := r.db.ExecContext(ctx,
		"UPDATE podsetnici SET zavrseno = ? WHERE id = ?", val, id,
	)
	if err != nil {
		return fmt.Errorf("ntech: PodsetnikRepo.OznaciZavrsenim: %w", err)
	}

	return nil
}

// Obrisi briše podsetnik po ID-u
func (r *PodsetnikRepo) Obrisi(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM podsetnici WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("ntech: PodsetnikRepo.Obrisi: %w", err)
	}

	return nil
}

// BrojAktivnih vraća broj nezavršenih podsetnika, opcionalno filtrirano po korisniku
func (r *PodsetnikRepo) BrojAktivnih(ctx context.Context, filter db.PodsetnikFilter) (int, error) {
	upit := "SELECT COUNT(*) FROM podsetnici WHERE zavrseno = 0"
	var args []any

	if filter.KorisnikID != nil {
		upit += " AND korisnik_id = ?"
		args = append(args, *filter.KorisnikID)
	}

	var broj int
	err := r.db.QueryRowContext(ctx, upit, args...).Scan(&broj)
	if err != nil {
		return 0, fmt.Errorf("ntech: PodsetnikRepo.BrojAktivnih: %w", err)
	}

	return broj, nil
}
