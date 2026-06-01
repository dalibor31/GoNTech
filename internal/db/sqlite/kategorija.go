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
	redovi, err := r.db.QueryContext(ctx, "SELECT id, naziv, opis FROM kategorije ORDER BY naziv ASC")
	if err != nil {
		return nil, fmt.Errorf("ntech: KategorijaRepo.Lista: %w", err)
	}
	defer redovi.Close()

	var rezultat []model.Kategorija
	for redovi.Next() {
		var k model.Kategorija
		var opis sql.NullString
		if err := redovi.Scan(&k.ID, &k.Naziv, &opis); err != nil {
			return nil, fmt.Errorf("ntech: KategorijaRepo.Lista: scan: %w", err)
		}
		if opis.Valid {
			k.Opis = opis.String
		}
		rezultat = append(rezultat, k)
	}

	return rezultat, nil
}

// Kreiraj dodaje novu kategoriju
func (r *KategorijaRepo) Kreiraj(ctx context.Context, k *model.Kategorija) (int64, error) {
	rezultat, err := r.db.ExecContext(ctx,
		"INSERT INTO kategorije (naziv, opis) VALUES (?, ?)",
		k.Naziv, k.Opis,
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
