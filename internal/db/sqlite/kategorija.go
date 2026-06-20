package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"ntech/internal/model"
)

// KategorijaRepo je SQLite implementacija KategorijaRepository interfejsa
type KategorijaRepo struct {
	db *sql.DB
}

// NovaKategorijaRepo kreira novi KategorijaRepo
func NovaKategorijaRepo(db *sql.DB) *KategorijaRepo {
	return &KategorijaRepo{db: db}
}

// Lista vraća sve kategorije
func (r *KategorijaRepo) Lista(ctx context.Context) ([]model.Kategorija, error) {
	redovi, err := r.db.QueryContext(ctx, "SELECT id, naziv, opis, kod, marza FROM kategorije ORDER BY naziv ASC")
	if err != nil {
		return nil, fmt.Errorf("ntech: KategorijaRepo.Lista: %w", err)
	}
	defer redovi.Close()

	var rezultat []model.Kategorija
	for redovi.Next() {
		var k model.Kategorija
		var opis, kod sql.NullString
		var marza sql.NullFloat64
		if err := redovi.Scan(&k.ID, &k.Naziv, &opis, &kod, &marza); err != nil {
			return nil, fmt.Errorf("ntech: KategorijaRepo.Lista: scan: %w", err)
		}
		if opis.Valid {
			k.Opis = opis.String
		}
		if kod.Valid {
			k.Kod = kod.String
		}
		if marza.Valid {
			k.Marza = &marza.Float64
		}
		rezultat = append(rezultat, k)
	}

	return rezultat, nil
}

// Kreiraj dodaje novu kategoriju
func (r *KategorijaRepo) Kreiraj(ctx context.Context, k *model.Kategorija) (int64, error) {
	var kod any
	if k.Kod != "" {
		kod = k.Kod
	}
	rezultat, err := r.db.ExecContext(ctx,
		"INSERT INTO kategorije (naziv, opis, kod, marza) VALUES (?, ?, ?, ?)",
		k.Naziv, k.Opis, kod, k.Marza,
	)
	if err != nil {
		return 0, fmt.Errorf("ntech: KategorijaRepo.Kreiraj: %w", err)
	}

	id, err := rezultat.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("ntech: KategorijaRepo.Kreiraj: last insert id: %w", err)
	}

	return id, nil
}

// DohvatiID vraća jednu kategoriju po ID-u
func (r *KategorijaRepo) DohvatiID(ctx context.Context, id int64) (*model.Kategorija, error) {
	var k model.Kategorija
	var opis, kod sql.NullString
	var marza sql.NullFloat64
	err := r.db.QueryRowContext(ctx,
		"SELECT id, naziv, opis, kod, marza FROM kategorije WHERE id = ?", id).
		Scan(&k.ID, &k.Naziv, &opis, &kod, &marza)
	if err != nil {
		return nil, fmt.Errorf("ntech: KategorijaRepo.DohvatiID: %w", err)
	}
	if opis.Valid {
		k.Opis = opis.String
	}
	if kod.Valid {
		k.Kod = kod.String
	}
	if marza.Valid {
		k.Marza = &marza.Float64
	}
	return &k, nil
}

// Izmeni ažurira naziv, opis i maržu postojeće kategorije
func (r *KategorijaRepo) Izmeni(ctx context.Context, k *model.Kategorija) error {
	var kod any
	if k.Kod != "" {
		kod = k.Kod
	}
	_, err := r.db.ExecContext(ctx,
		"UPDATE kategorije SET naziv = ?, opis = ?, kod = ?, marza = ? WHERE id = ?",
		k.Naziv, k.Opis, kod, k.Marza, k.ID,
	)
	if err != nil {
		return fmt.Errorf("ntech: KategorijaRepo.Izmeni: %w", err)
	}
	return nil
}
