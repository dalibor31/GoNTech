package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"ntech/internal/model"
)

// KlijentRepo je SQLite implementacija KlijentRepository interfejsa
type KlijentRepo struct {
	db *sql.DB
}

// NoviKlijentRepo kreira novi KlijentRepo
func NoviKlijentRepo(db *sql.DB) *KlijentRepo {
	return &KlijentRepo{db: db}
}

// Lista vraća listu klijenata sa opcionom pretragom po imenu, prezimenu ili nazivu firme
func (r *KlijentRepo) Lista(ctx context.Context, pretraga string) ([]model.Klijent, error) {
	upit := `
		SELECT id, ime, prezime, naziv_firme, pib, telefon, email, napomena, datum_unosa
		FROM klijenti
		WHERE 1=1`

	args := []any{}

	if pretraga != "" {
		upit += " AND (ime LIKE ? OR prezime LIKE ? OR naziv_firme LIKE ?)"
		p := "%" + pretraga + "%"
		args = append(args, p, p, p)
	}

	upit += " ORDER BY datum_unosa DESC"

	redovi, err := r.db.QueryContext(ctx, upit, args...)
	if err != nil {
		return nil, fmt.Errorf("ntech: KlijentRepo.Lista: %w", err)
	}
	defer redovi.Close()

	var rezultat []model.Klijent
	for redovi.Next() {
		var k model.Klijent
		var ime, prezime, nazivFirme, pib, telefon, email, napomena sql.NullString
		err := redovi.Scan(
			&k.ID, &ime, &prezime, &nazivFirme, &pib, &telefon, &email, &napomena, &k.DatumUnosa,
		)
		if err != nil {
			return nil, fmt.Errorf("ntech: KlijentRepo.Lista: scan: %w", err)
		}
		k.Ime = ime.String
		k.Prezime = prezime.String
		k.NazivFirme = nazivFirme.String
		k.PIB = pib.String
		k.Telefon = telefon.String
		k.Email = email.String
		k.Napomena = napomena.String
		rezultat = append(rezultat, k)
	}

	return rezultat, nil
}

// DohvatiID vraća jednog klijenta po ID-u
func (r *KlijentRepo) DohvatiID(ctx context.Context, id int64) (*model.Klijent, error) {
	var k model.Klijent
	var ime, prezime, nazivFirme, pib, telefon, email, napomena sql.NullString

	err := r.db.QueryRowContext(ctx, `
		SELECT id, ime, prezime, naziv_firme, pib, telefon, email, napomena, datum_unosa
		FROM klijenti WHERE id = ?`, id).Scan(
		&k.ID, &ime, &prezime, &nazivFirme, &pib, &telefon, &email, &napomena, &k.DatumUnosa,
	)
	if err != nil {
		return nil, fmt.Errorf("ntech: KlijentRepo.DohvatiID: %w", err)
	}

	k.Ime = ime.String
	k.Prezime = prezime.String
	k.NazivFirme = nazivFirme.String
	k.PIB = pib.String
	k.Telefon = telefon.String
	k.Email = email.String
	k.Napomena = napomena.String

	return &k, nil
}

// Kreiraj dodaje novog klijenta u bazu
func (r *KlijentRepo) Kreiraj(ctx context.Context, k *model.Klijent) (int64, error) {
	rezultat, err := r.db.ExecContext(ctx, `
		INSERT INTO klijenti (ime, prezime, naziv_firme, pib, telefon, email, napomena)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		nullString(k.Ime), nullString(k.Prezime), nullString(k.NazivFirme),
		nullString(k.PIB), nullString(k.Telefon), nullString(k.Email), nullString(k.Napomena),
	)
	if err != nil {
		return 0, fmt.Errorf("ntech: KlijentRepo.Kreiraj: %w", err)
	}

	id, err := rezultat.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("ntech: KlijentRepo.Kreiraj: last insert id: %w", err)
	}

	return id, nil
}

// Izmeni ažurira postojećeg klijenta
func (r *KlijentRepo) Izmeni(ctx context.Context, k *model.Klijent) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE klijenti SET
			ime = ?, prezime = ?, naziv_firme = ?, pib = ?, telefon = ?, email = ?, napomena = ?
		WHERE id = ?`,
		nullString(k.Ime), nullString(k.Prezime), nullString(k.NazivFirme),
		nullString(k.PIB), nullString(k.Telefon), nullString(k.Email), nullString(k.Napomena),
		k.ID,
	)
	if err != nil {
		return fmt.Errorf("ntech: KlijentRepo.Izmeni: %w", err)
	}

	return nil
}

// Obrisi briše klijenta po ID-u
func (r *KlijentRepo) Obrisi(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM klijenti WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("ntech: KlijentRepo.Obrisi: %w", err)
	}

	return nil
}
