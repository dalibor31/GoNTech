package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"ntech/internal/model"
)

// PdvStopaRepo je SQLite implementacija PdvStopaRepository interfejsa
type PdvStopaRepo struct {
	db *sql.DB
}

// NoviPdvStopaRepo kreira novi PdvStopaRepo
func NoviPdvStopaRepo(db *sql.DB) *PdvStopaRepo {
	return &PdvStopaRepo{db: db}
}

// Lista vraća stope iz šifarnika; ako je samoAktivne true, izostavlja arhivirane
func (r *PdvStopaRepo) Lista(ctx context.Context, samoAktivne bool) ([]model.PdvStopa, error) {
	upit := `
		SELECT id, naziv, stopa, oznaka, aktivna, redosled, datum_unosa
		FROM pdv_stope
		WHERE 1=1`
	if samoAktivne {
		upit += " AND aktivna = 1"
	}
	upit += " ORDER BY redosled ASC, stopa DESC"

	redovi, err := r.db.QueryContext(ctx, upit)
	if err != nil {
		return nil, fmt.Errorf("ntech: PdvStopaRepo.Lista: %w", err)
	}
	defer redovi.Close()

	var rezultat []model.PdvStopa
	for redovi.Next() {
		var s model.PdvStopa
		if err := redovi.Scan(&s.ID, &s.Naziv, &s.Stopa, &s.Oznaka, &s.Aktivna, &s.Redosled, &s.DatumUnosa); err != nil {
			return nil, fmt.Errorf("ntech: PdvStopaRepo.Lista: scan: %w", err)
		}
		rezultat = append(rezultat, s)
	}
	if err := redovi.Err(); err != nil {
		return nil, fmt.Errorf("ntech: PdvStopaRepo.Lista: %w", err)
	}
	return rezultat, nil
}

// DohvatiID vraća jednu stopu po identifikatoru
func (r *PdvStopaRepo) DohvatiID(ctx context.Context, id int64) (*model.PdvStopa, error) {
	var s model.PdvStopa
	err := r.db.QueryRowContext(ctx, `
		SELECT id, naziv, stopa, oznaka, aktivna, redosled, datum_unosa
		FROM pdv_stope WHERE id = ?`, id).
		Scan(&s.ID, &s.Naziv, &s.Stopa, &s.Oznaka, &s.Aktivna, &s.Redosled, &s.DatumUnosa)
	if err != nil {
		return nil, fmt.Errorf("ntech: PdvStopaRepo.DohvatiID: %w", err)
	}
	return &s, nil
}

// Kreiraj dodaje novu stopu i vraća njen id
func (r *PdvStopaRepo) Kreiraj(ctx context.Context, s *model.PdvStopa) (int64, error) {
	rez, err := r.db.ExecContext(ctx, `
		INSERT INTO pdv_stope (naziv, stopa, oznaka, aktivna, redosled)
		VALUES (?, ?, ?, ?, ?)`,
		s.Naziv, s.Stopa, s.Oznaka, s.Aktivna, s.Redosled)
	if err != nil {
		return 0, fmt.Errorf("ntech: PdvStopaRepo.Kreiraj: %w", err)
	}
	id, err := rez.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("ntech: PdvStopaRepo.Kreiraj: id: %w", err)
	}
	return id, nil
}

// Izmeni menja podatke postojeće stope (osim datuma unosa)
func (r *PdvStopaRepo) Izmeni(ctx context.Context, s *model.PdvStopa) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE pdv_stope
		SET naziv = ?, stopa = ?, oznaka = ?, aktivna = ?, redosled = ?
		WHERE id = ?`,
		s.Naziv, s.Stopa, s.Oznaka, s.Aktivna, s.Redosled, s.ID)
	if err != nil {
		return fmt.Errorf("ntech: PdvStopaRepo.Izmeni: %w", err)
	}
	return nil
}

// PostaviAktivnu arhivira (false) ili vraća u upotrebu (true) stopu, bez brisanja
func (r *PdvStopaRepo) PostaviAktivnu(ctx context.Context, id int64, aktivna bool) error {
	_, err := r.db.ExecContext(ctx, "UPDATE pdv_stope SET aktivna = ? WHERE id = ?", aktivna, id)
	if err != nil {
		return fmt.Errorf("ntech: PdvStopaRepo.PostaviAktivnu: %w", err)
	}
	return nil
}
