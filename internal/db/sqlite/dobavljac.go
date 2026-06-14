package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"ntech/internal/model"
)

// DobavljacRepo je SQLite implementacija DobavljacRepository interfejsa
type DobavljacRepo struct {
	db *sql.DB
}

// NoviDobavljacRepo kreira novi DobavljacRepo
func NoviDobavljacRepo(db *sql.DB) *DobavljacRepo {
	return &DobavljacRepo{db: db}
}

// Lista vraća listu dobavljača sa opcionom pretragom po nazivu
func (r *DobavljacRepo) Lista(ctx context.Context, pretraga string) ([]model.Dobavljac, error) {
	upit := `
		SELECT id, naziv, kontakt_osoba, telefon, email, pib, mesto, napomena, datum_unosa
		FROM dobavljaci
		WHERE 1=1`

	args := []any{}

	if pretraga != "" {
		upit += " AND naziv LIKE ?"
		args = append(args, "%"+pretraga+"%")
	}

	upit += " ORDER BY naziv ASC"

	redovi, err := r.db.QueryContext(ctx, upit, args...)
	if err != nil {
		return nil, fmt.Errorf("ntech: DobavljacRepo.Lista: %w", err)
	}
	defer redovi.Close()

	var rezultat []model.Dobavljac
	for redovi.Next() {
		var d model.Dobavljac
		var kontaktOsoba, telefon, email, pib, mesto, napomena sql.NullString
		err := redovi.Scan(
			&d.ID, &d.Naziv, &kontaktOsoba, &telefon, &email, &pib, &mesto, &napomena, &d.DatumUnosa,
		)
		if err != nil {
			return nil, fmt.Errorf("ntech: DobavljacRepo.Lista: scan: %w", err)
		}
		d.KontaktOsoba = kontaktOsoba.String
		d.Telefon = telefon.String
		d.Email = email.String
		d.PIB = pib.String
		d.Mesto = mesto.String
		d.Napomena = napomena.String
		rezultat = append(rezultat, d)
	}

	return rezultat, nil
}

// DohvatiID vraća jednog dobavljača po ID-u
func (r *DobavljacRepo) DohvatiID(ctx context.Context, id int64) (*model.Dobavljac, error) {
	var d model.Dobavljac
	var kontaktOsoba, telefon, email, pib, mesto, napomena sql.NullString

	err := r.db.QueryRowContext(ctx, `
		SELECT id, naziv, kontakt_osoba, telefon, email, pib, mesto, napomena, datum_unosa
		FROM dobavljaci WHERE id = ?`, id).Scan(
		&d.ID, &d.Naziv, &kontaktOsoba, &telefon, &email, &pib, &mesto, &napomena, &d.DatumUnosa,
	)
	if err != nil {
		return nil, fmt.Errorf("ntech: DobavljacRepo.DohvatiID: %w", err)
	}

	d.KontaktOsoba = kontaktOsoba.String
	d.Telefon = telefon.String
	d.Email = email.String
	d.PIB = pib.String
	d.Mesto = mesto.String
	d.Napomena = napomena.String

	return &d, nil
}

// Kreiraj dodaje novog dobavljača u bazu
func (r *DobavljacRepo) Kreiraj(ctx context.Context, d *model.Dobavljac) (int64, error) {
	rezultat, err := r.db.ExecContext(ctx, `
		INSERT INTO dobavljaci (naziv, kontakt_osoba, telefon, email, pib, mesto, napomena)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		d.Naziv, nullString(d.KontaktOsoba), nullString(d.Telefon),
		nullString(d.Email), nullString(d.PIB), nullString(d.Mesto), nullString(d.Napomena),
	)
	if err != nil {
		return 0, fmt.Errorf("ntech: DobavljacRepo.Kreiraj: %w", err)
	}

	id, err := rezultat.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("ntech: DobavljacRepo.Kreiraj: last insert id: %w", err)
	}

	return id, nil
}

// Izmeni ažurira postojećeg dobavljača
func (r *DobavljacRepo) Izmeni(ctx context.Context, d *model.Dobavljac) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE dobavljaci SET
			naziv = ?, kontakt_osoba = ?, telefon = ?, email = ?, pib = ?, mesto = ?, napomena = ?
		WHERE id = ?`,
		d.Naziv, nullString(d.KontaktOsoba), nullString(d.Telefon),
		nullString(d.Email), nullString(d.PIB), nullString(d.Mesto), nullString(d.Napomena), d.ID,
	)
	if err != nil {
		return fmt.Errorf("ntech: DobavljacRepo.Izmeni: %w", err)
	}

	return nil
}

// Obrisi briše dobavljača po ID-u
func (r *DobavljacRepo) Obrisi(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM dobavljaci WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("ntech: DobavljacRepo.Obrisi: %w", err)
	}

	return nil
}
