package sqlite

import (
	"context"
	"database/sql"
	"errors"
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
		SELECT id, tip, ime, prezime, jmbg, tip_identifikacije, naziv_firme, pib, telefon, email, mesto, adresa, napomena, datum_unosa
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
		var ime, prezime, jmbg, tipIdent, nazivFirme, pib, telefon, email, mesto, adresa, napomena sql.NullString
		err := redovi.Scan(
			&k.ID, &k.Tip, &ime, &prezime, &jmbg, &tipIdent, &nazivFirme, &pib, &telefon, &email, &mesto, &adresa, &napomena, &k.DatumUnosa,
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
		k.Adresa = adresa.String
		k.Napomena = napomena.String
		rezultat = append(rezultat, k)
	}

	return rezultat, nil
}

// DohvatiID vraća jednog klijenta po ID-u
func (r *KlijentRepo) DohvatiID(ctx context.Context, id int64) (*model.Klijent, error) {
	var k model.Klijent
	var ime, prezime, jmbg, tipIdent, nazivFirme, pib, telefon, email, mesto, adresa, napomena sql.NullString

	err := r.db.QueryRowContext(ctx, `
		SELECT id, tip, ime, prezime, jmbg, tip_identifikacije, naziv_firme, pib, telefon, email, mesto, adresa, napomena, datum_unosa
		FROM klijenti WHERE id = ?`, id).Scan(
		&k.ID, &k.Tip, &ime, &prezime, &jmbg, &tipIdent, &nazivFirme, &pib, &telefon, &email, &mesto, &adresa, &napomena, &k.DatumUnosa,
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
	k.Adresa = adresa.String
	k.Napomena = napomena.String

	return &k, nil
}

// Pronadji traži postojećeg klijenta po imenu i prezimenu (fizičko) ili nazivu firme (pravno).
// Vraća nil, nil ako nije pronađen.
// Pretraga ide od najočiglednijeg ka opštijem: JMBG/PIB → ime+prezime+mesto → telefon → email → samo ime+prezime.
func (r *KlijentRepo) Pronadji(ctx context.Context, tip, ime, prezime, nazivFirme, jmbg, telefon, email, mesto string) (*model.Klijent, error) {
	var k model.Klijent
	var imeN, prezimeN, jmbgN, tipIdent, nazivFirmeN, pibN, telefonN, emailN, mestoN, adresaN, napomena sql.NullString

	skeniraj := func(row *sql.Row) error {
		return row.Scan(
			&k.ID, &k.Tip, &imeN, &prezimeN, &jmbgN, &tipIdent, &nazivFirmeN, &pibN, &telefonN, &emailN, &mestoN, &adresaN, &napomena, &k.DatumUnosa,
		)
	}

	var row *sql.Row

	if tip == "pravno" {
		// 1. PIB (najpreciznije za firmu)
		if jmbg != "" {
			row = r.db.QueryRowContext(ctx, `
				SELECT id, tip, ime, prezime, jmbg, tip_identifikacije, naziv_firme, pib, telefon, email, mesto, adresa, napomena, datum_unosa
				FROM klijenti WHERE tip = 'pravno' AND pib = ? LIMIT 1`, jmbg)
			if err := skeniraj(row); err == nil {
				goto popuni
			}
		}
		// 2. Naziv firme + mesto
		if nazivFirme != "" && mesto != "" {
			row = r.db.QueryRowContext(ctx, `
				SELECT id, tip, ime, prezime, jmbg, tip_identifikacije, naziv_firme, pib, telefon, email, mesto, adresa, napomena, datum_unosa
				FROM klijenti WHERE tip = 'pravno' AND naziv_firme = ? AND mesto = ? LIMIT 1`, nazivFirme, mesto)
			if err := skeniraj(row); err == nil {
				goto popuni
			}
		}
		// 3. Samo naziv firme
		row = r.db.QueryRowContext(ctx, `
			SELECT id, tip, ime, prezime, jmbg, tip_identifikacije, naziv_firme, pib, telefon, email, mesto, adresa, napomena, datum_unosa
			FROM klijenti WHERE tip = 'pravno' AND naziv_firme = ? LIMIT 1`, nazivFirme)
		if err := skeniraj(row); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("ntech: KlijentRepo.Pronadji: %w", err)
		}
	} else {
		// Fizičko lice — više nivoa
		// 1. JMBG ili broj lične karte
		if jmbg != "" {
			row = r.db.QueryRowContext(ctx, `
				SELECT id, tip, ime, prezime, jmbg, tip_identifikacije, naziv_firme, pib, telefon, email, mesto, adresa, napomena, datum_unosa
				FROM klijenti WHERE tip != 'pravno' AND jmbg = ? LIMIT 1`, jmbg)
			if err := skeniraj(row); err == nil {
				goto popuni
			}
		}
		// 2. Ime + prezime + mesto
		if ime != "" && prezime != "" && mesto != "" {
			row = r.db.QueryRowContext(ctx, `
				SELECT id, tip, ime, prezime, jmbg, tip_identifikacije, naziv_firme, pib, telefon, email, mesto, adresa, napomena, datum_unosa
				FROM klijenti WHERE tip != 'pravno' AND ime = ? AND prezime = ? AND mesto = ? LIMIT 1`, ime, prezime, mesto)
			if err := skeniraj(row); err == nil {
				goto popuni
			}
		}
		// 3. Ime + prezime + telefon
		if ime != "" && prezime != "" && telefon != "" {
			row = r.db.QueryRowContext(ctx, `
				SELECT id, tip, ime, prezime, jmbg, tip_identifikacije, naziv_firme, pib, telefon, email, mesto, adresa, napomena, datum_unosa
				FROM klijenti WHERE tip != 'pravno' AND ime = ? AND prezime = ? AND telefon = ? LIMIT 1`, ime, prezime, telefon)
			if err := skeniraj(row); err == nil {
				goto popuni
			}
		}
		// 4. Ime + prezime + email
		if ime != "" && prezime != "" && email != "" {
			row = r.db.QueryRowContext(ctx, `
				SELECT id, tip, ime, prezime, jmbg, tip_identifikacije, naziv_firme, pib, telefon, email, mesto, adresa, napomena, datum_unosa
				FROM klijenti WHERE tip != 'pravno' AND ime = ? AND prezime = ? AND email = ? LIMIT 1`, ime, prezime, email)
			if err := skeniraj(row); err == nil {
				goto popuni
			}
		}
		// 5. Samo ime + prezime (poslednji fallback)
		row = r.db.QueryRowContext(ctx, `
			SELECT id, tip, ime, prezime, jmbg, tip_identifikacije, naziv_firme, pib, telefon, email, mesto, adresa, napomena, datum_unosa
			FROM klijenti WHERE tip != 'pravno' AND ime = ? AND prezime = ? LIMIT 1`, ime, prezime)
		if err := skeniraj(row); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("ntech: KlijentRepo.Pronadji: %w", err)
		}
	}

popuni:
	k.Ime = imeN.String
	k.Prezime = prezimeN.String
	k.JMBG = jmbgN.String
	k.TipIdentifikacije = tipIdent.String
	k.NazivFirme = nazivFirmeN.String
	k.PIB = pibN.String
	k.Telefon = telefonN.String
	k.Email = emailN.String
	k.Mesto = mestoN.String
	k.Adresa = adresaN.String
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
		INSERT INTO klijenti (tip, ime, prezime, jmbg, tip_identifikacije, naziv_firme, pib, telefon, email, mesto, adresa, napomena)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		k.Tip, nullString(k.Ime), nullString(k.Prezime), nullString(k.JMBG), k.TipIdentifikacije,
		nullString(k.NazivFirme), nullString(k.PIB), nullString(k.Telefon),
		nullString(k.Email), nullString(k.Mesto), nullString(k.Adresa), nullString(k.Napomena),
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
			pib = ?, telefon = ?, email = ?, mesto = ?, adresa = ?, napomena = ?
		WHERE id = ?`,
		k.Tip, nullString(k.Ime), nullString(k.Prezime), nullString(k.JMBG), k.TipIdentifikacije,
		nullString(k.NazivFirme), nullString(k.PIB), nullString(k.Telefon),
		nullString(k.Email), nullString(k.Mesto), nullString(k.Adresa), nullString(k.Napomena), k.ID,
	)
	if err != nil {
		return fmt.Errorf("ntech: KlijentRepo.Izmeni: %w", err)
	}

	return nil
}

// ListaFilter vraća listu klijenata sa limitom i offsetom (paginacija)
func (r *KlijentRepo) ListaFilter(ctx context.Context, filter db.KlijentFilter) ([]model.Klijent, error) {
	upit := `
		SELECT id, tip, ime, prezime, jmbg, tip_identifikacije, naziv_firme, pib, telefon, email, mesto, adresa, napomena, datum_unosa
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
		var ime, prezime, jmbg, tipIdent, nazivFirme, pib, telefon, email, mesto, adresa, napomena sql.NullString
		err := redovi.Scan(
			&k.ID, &k.Tip, &ime, &prezime, &jmbg, &tipIdent, &nazivFirme, &pib, &telefon, &email, &mesto, &adresa, &napomena, &k.DatumUnosa,
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
		k.Adresa = adresa.String
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
