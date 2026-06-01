package db

import (
	"context"

	"ntech/internal/model"
)

// ArtikalRepository definiše operacije nad artiklima
type ArtikalRepository interface {
	Lista(ctx context.Context, filter ArtikalFilter) ([]model.ArtikalSaKategorijom, error)
	DohvatiID(ctx context.Context, id int64) (*model.Artikal, error)
	Kreiraj(ctx context.Context, a *model.Artikal) (int64, error)
	Izmeni(ctx context.Context, a *model.Artikal) error
	Obrisi(ctx context.Context, id int64) error
}

// KategorijaRepository definiše operacije nad kategorijama
type KategorijaRepository interface {
	Lista(ctx context.Context) ([]model.Kategorija, error)
	Kreiraj(ctx context.Context, k *model.Kategorija) (int64, error)
}

// ArtikalFilter definiše parametre za filtriranje liste artikala
type ArtikalFilter struct {
	Pretraga       string
	KategorijaID   *int64
	SamoKriticni   bool
}
