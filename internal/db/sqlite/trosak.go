package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"ntech/internal/db"
	"ntech/internal/model"
)

// TrosakRepo je SQLite implementacija db.TrosakRepository
type TrosakRepo struct {
	db *sql.DB
}

// NoviTrosakRepo kreira repozitorijum troškova nad datom bazom
func NoviTrosakRepo(baza *sql.DB) *TrosakRepo {
	return &TrosakRepo{db: baza}
}

// skenirajTrosak čita jedan red u model.Trosak
func skenirajTrosak(s interface {
	Scan(dest ...any) error
}) (model.Trosak, error) {
	var t model.Trosak
	var sifra sql.NullString
	var arhiviran int
	var datumUnosa string
	if err := s.Scan(&t.ID, &sifra, &t.Naziv, &t.Cena, &t.Opis, &arhiviran, &datumUnosa); err != nil {
		return t, err
	}
	t.Sifra = sifra.String
	t.Arhiviran = arhiviran == 1
	if parsed, err := parseDatumUnosa(datumUnosa); err == nil {
		t.DatumUnosa = parsed
	}
	return t, nil
}

const trosakKolone = "id, sifra, naziv, cena, opis, arhiviran, datum_unosa"

// Lista vraća troškove prema filteru (pretraga po nazivu/šifri, aktivne/arhivirane)
func (r *TrosakRepo) Lista(ctx context.Context, filter db.TrosakFilter) ([]model.Trosak, error) {
	upit := "SELECT " + trosakKolone + " FROM troskovi WHERE 1=1"
	var args []any

	if filter.Pretraga != "" {
		upit += " AND (naziv LIKE ? OR sifra LIKE ?)"
		t := "%" + filter.Pretraga + "%"
		args = append(args, t, t)
	}
	if filter.Arhivirani {
		upit += " AND arhiviran = 1"
	} else {
		upit += " AND arhiviran = 0"
	}
	upit += " ORDER BY naziv"
	if filter.Limit > 0 {
		upit += " LIMIT ? OFFSET ?"
		args = append(args, filter.Limit, filter.Offset)
	}

	redovi, err := r.db.QueryContext(ctx, upit, args...)
	if err != nil {
		return nil, fmt.Errorf("ntech: TrosakRepo.Lista: %w", err)
	}
	defer redovi.Close()

	var troskovi []model.Trosak
	for redovi.Next() {
		t, err := skenirajTrosak(redovi)
		if err != nil {
			return nil, fmt.Errorf("ntech: TrosakRepo.Lista: %w", err)
		}
		troskovi = append(troskovi, t)
	}
	return troskovi, redovi.Err()
}

// DohvatiID vraća jedan trošak po ID-u
func (r *TrosakRepo) DohvatiID(ctx context.Context, id int64) (*model.Trosak, error) {
	red := r.db.QueryRowContext(ctx, "SELECT "+trosakKolone+" FROM troskovi WHERE id = ?", id)
	t, err := skenirajTrosak(red)
	if err != nil {
		return nil, fmt.Errorf("ntech: TrosakRepo.DohvatiID: %w", err)
	}
	return &t, nil
}

// Kreiraj upisuje nov trošak i vraća njegov ID
func (r *TrosakRepo) Kreiraj(ctx context.Context, t *model.Trosak) (int64, error) {
	var sifra any
	if t.Sifra != "" {
		sifra = t.Sifra
	}
	rez, err := r.db.ExecContext(ctx, `
		INSERT INTO troskovi (sifra, naziv, cena, opis)
		VALUES (?, ?, ?, ?)`,
		sifra, t.Naziv, t.Cena, t.Opis,
	)
	if err != nil {
		return 0, fmt.Errorf("ntech: TrosakRepo.Kreiraj: %w", err)
	}
	id, err := rez.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("ntech: TrosakRepo.Kreiraj: last insert id: %w", err)
	}
	return id, nil
}

// Izmeni ažurira postojeći trošak
func (r *TrosakRepo) Izmeni(ctx context.Context, t *model.Trosak) error {
	var sifra any
	if t.Sifra != "" {
		sifra = t.Sifra
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE troskovi SET sifra = ?, naziv = ?, cena = ?, opis = ?
		WHERE id = ?`,
		sifra, t.Naziv, t.Cena, t.Opis, t.ID,
	)
	if err != nil {
		return fmt.Errorf("ntech: TrosakRepo.Izmeni: %w", err)
	}
	return nil
}

// Obrisi trajno briše trošak po ID-u
func (r *TrosakRepo) Obrisi(ctx context.Context, id int64) error {
	if _, err := r.db.ExecContext(ctx, "DELETE FROM troskovi WHERE id = ?", id); err != nil {
		return fmt.Errorf("ntech: TrosakRepo.Obrisi: %w", err)
	}
	return nil
}

// SledecaSifra vraća predlog sledeće auto-šifre troška u formatu TRO-001 … TRO-999
func (r *TrosakRepo) SledecaSifra(ctx context.Context) (string, error) {
	var maks sql.NullInt64
	err := r.db.QueryRowContext(ctx,
		"SELECT MAX(CAST(SUBSTR(sifra, 5) AS INTEGER)) FROM troskovi WHERE sifra LIKE 'TRO-%'").Scan(&maks)
	if err != nil {
		return "TRO-001", fmt.Errorf("ntech: TrosakRepo.SledecaSifra: %w", err)
	}
	return fmt.Sprintf("TRO-%03d", maks.Int64+1), nil
}
