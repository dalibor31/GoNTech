package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"ntech/internal/db"
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
		SELECT id, tip, ime, prezime, jmbg, tip_identifikacije, naziv_firme, pib, telefon, email, mesto, napomena, datum_unosa
		FROM klijenti
		WHERE 1=1`

	args := []any{}

	if pretraga != "" {
		upit += " AND (ime LIKE ? OR prezime LIKE ? OR (ime || ' ' || prezime) LIKE ? OR naziv_firme LIKE ? OR telefon LIKE ? OR email LIKE ?)"
		p := "%" + pretraga + "%"
		args = append(args, p, p, p, p, p, p)
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
		var ime, prezime, jmbg, tipIdent, nazivFirme, pib, telefon, email, mesto, napomena sql.NullString
		err := redovi.Scan(
			&k.ID, &k.Tip, &ime, &prezime, &jmbg, &tipIdent, &nazivFirme, &pib, &telefon, &email, &mesto, &napomena, &k.DatumUnosa,
		)
		if err != nil {
			return nil, fmt.Errorf("ntech: KlijentRepo.Lista: scan: %w", err)
		}
		k.Ime = ime.String
		k.Prezime = prezime.String
		k.JMBG = jmbg.String
		k.TipIdentifikacije = tipIdent.String
		k.NazivFirme = nazivFirme.String
		k.PIB = pib.String
		k.Telefon = telefon.String
		k.Email = email.String
		k.Mesto = mesto.String
		k.Napomena = napomena.String
		rezultat = append(rezultat, k)
	}

	return rezultat, nil
}

// DohvatiID vraća jednog klijenta po ID-u
func (r *KlijentRepo) DohvatiID(ctx context.Context, id int64) (*model.Klijent, error) {
	var k model.Klijent
	var ime, prezime, jmbg, tipIdent, nazivFirme, pib, telefon, email, mesto, napomena sql.NullString

	err := r.db.QueryRowContext(ctx, `
		SELECT id, tip, ime, prezime, jmbg, tip_identifikacije, naziv_firme, pib, telefon, email, mesto, napomena, datum_unosa
		FROM klijenti WHERE id = ?`, id).Scan(
		&k.ID, &k.Tip, &ime, &prezime, &jmbg, &tipIdent, &nazivFirme, &pib, &telefon, &email, &mesto, &napomena, &k.DatumUnosa,
	)
	if err != nil {
		return nil, fmt.Errorf("ntech: KlijentRepo.DohvatiID: %w", err)
	}

	k.Ime = ime.String
	k.Prezime = prezime.String
	k.JMBG = jmbg.String
	k.TipIdentifikacije = tipIdent.String
	k.NazivFirme = nazivFirme.String
	k.PIB = pib.String
	k.Telefon = telefon.String
	k.Email = email.String
	k.Mesto = mesto.String
	k.Napomena = napomena.String

	return &k, nil
}

// Kreiraj dodaje novog klijenta u bazu
func (r *KlijentRepo) Kreiraj(ctx context.Context, k *model.Klijent) (int64, error) {
	if k.Tip == "" {
		k.Tip = "fizicko"
	}
	if k.TipIdentifikacije == "" {
		k.TipIdentifikacije = "jmbg"
	}
	rezultat, err := r.db.ExecContext(ctx, `
		INSERT INTO klijenti (tip, ime, prezime, jmbg, tip_identifikacije, naziv_firme, pib, telefon, email, mesto, napomena)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		k.Tip, nullString(k.Ime), nullString(k.Prezime), nullString(k.JMBG), k.TipIdentifikacije,
		nullString(k.NazivFirme), nullString(k.PIB), nullString(k.Telefon),
		nullString(k.Email), nullString(k.Mesto), nullString(k.Napomena),
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
	if k.Tip == "" {
		k.Tip = "fizicko"
	}
	if k.TipIdentifikacije == "" {
		k.TipIdentifikacije = "jmbg"
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE klijenti SET
			tip = ?, ime = ?, prezime = ?, jmbg = ?, tip_identifikacije = ?, naziv_firme = ?,
			pib = ?, telefon = ?, email = ?, mesto = ?, napomena = ?
		WHERE id = ?`,
		k.Tip, nullString(k.Ime), nullString(k.Prezime), nullString(k.JMBG), k.TipIdentifikacije,
		nullString(k.NazivFirme), nullString(k.PIB), nullString(k.Telefon),
		nullString(k.Email), nullString(k.Mesto), nullString(k.Napomena), k.ID,
	)
	if err != nil {
		return fmt.Errorf("ntech: KlijentRepo.Izmeni: %w", err)
	}

	return nil
}

// ListaFilter vraća listu klijenata sa limitom i offsetom (paginacija)
func (r *KlijentRepo) ListaFilter(ctx context.Context, filter db.KlijentFilter) ([]model.Klijent, error) {
	upit := `
		SELECT id, tip, ime, prezime, jmbg, tip_identifikacije, naziv_firme, pib, telefon, email, mesto, napomena, datum_unosa
		FROM klijenti
		WHERE 1=1`

	args := []any{}

	if filter.Pretraga != "" {
		upit += " AND (ime LIKE ? OR prezime LIKE ? OR (ime || ' ' || prezime) LIKE ? OR naziv_firme LIKE ? OR telefon LIKE ? OR email LIKE ?)"
		p := "%" + filter.Pretraga + "%"
		args = append(args, p, p, p, p, p, p)
	}
	if filter.Tip == "fizicko" || filter.Tip == "pravno" {
		upit += " AND tip = ?"
		args = append(args, filter.Tip)
	}

	upit += " ORDER BY datum_unosa DESC"

	if filter.Limit > 0 {
		upit += " LIMIT ?"
		args = append(args, filter.Limit)
		if filter.Offset > 0 {
			upit += " OFFSET ?"
			args = append(args, filter.Offset)
		}
	}

	redovi, err := r.db.QueryContext(ctx, upit, args...)
	if err != nil {
		return nil, fmt.Errorf("ntech: KlijentRepo.ListaFilter: %w", err)
	}
	defer redovi.Close()

	var rezultat []model.Klijent
	for redovi.Next() {
		var k model.Klijent
		var ime, prezime, jmbg, tipIdent, nazivFirme, pib, telefon, email, mesto, napomena sql.NullString
		err := redovi.Scan(
			&k.ID, &k.Tip, &ime, &prezime, &jmbg, &tipIdent, &nazivFirme, &pib, &telefon, &email, &mesto, &napomena, &k.DatumUnosa,
		)
		if err != nil {
			return nil, fmt.Errorf("ntech: KlijentRepo.ListaFilter: scan: %w", err)
		}
		k.Ime = ime.String
		k.Prezime = prezime.String
		k.JMBG = jmbg.String
		k.TipIdentifikacije = tipIdent.String
		k.NazivFirme = nazivFirme.String
		k.PIB = pib.String
		k.Telefon = telefon.String
		k.Email = email.String
		k.Mesto = mesto.String
		k.Napomena = napomena.String
		rezultat = append(rezultat, k)
	}

	return rezultat, nil
}

// PrebrojiPoFilteru vraća broj klijenata koji zadovoljavaju filter
func (r *KlijentRepo) PrebrojiPoFilteru(ctx context.Context, filter db.KlijentFilter) (int, error) {
	upit := `SELECT COUNT(*) FROM klijenti WHERE 1=1`
	args := []any{}

	if filter.Pretraga != "" {
		upit += " AND (ime LIKE ? OR prezime LIKE ? OR (ime || ' ' || prezime) LIKE ? OR naziv_firme LIKE ? OR telefon LIKE ? OR email LIKE ?)"
		p := "%" + filter.Pretraga + "%"
		args = append(args, p, p, p, p, p, p)
	}
	if filter.Tip == "fizicko" || filter.Tip == "pravno" {
		upit += " AND tip = ?"
		args = append(args, filter.Tip)
	}

	var broj int
	if err := r.db.QueryRowContext(ctx, upit, args...).Scan(&broj); err != nil {
		return 0, fmt.Errorf("ntech: KlijentRepo.PrebrojiPoFilteru: %w", err)
	}
	return broj, nil
}

// Obrisi briše klijenta po ID-u
func (r *KlijentRepo) Obrisi(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM klijenti WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("ntech: KlijentRepo.Obrisi: %w", err)
	}

	return nil
}
