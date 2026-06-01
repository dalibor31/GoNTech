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
	Pretraga     string
	KategorijaID *int64
	SamoKriticni bool
}

// NabavkaRepository definiše operacije nad nabavkama
type NabavkaRepository interface {
	Lista(ctx context.Context) ([]model.NabavkaSaDetaljem, error)
	DohvatiID(ctx context.Context, id int64) (*model.Nabavka, error)
	DohvatiStavke(ctx context.Context, nabavkaID int64) ([]model.StavkaSaArtiklom, error)
	Kreiraj(ctx context.Context, n *model.Nabavka, stavke []model.StavkaNabavke) (int64, error)
	Obrisi(ctx context.Context, id int64) error
}

// DobavljacRepository definiše operacije nad dobavljačima
type DobavljacRepository interface {
	Lista(ctx context.Context, pretraga string) ([]model.Dobavljac, error)
	DohvatiID(ctx context.Context, id int64) (*model.Dobavljac, error)
	Kreiraj(ctx context.Context, d *model.Dobavljac) (int64, error)
	Izmeni(ctx context.Context, d *model.Dobavljac) error
	Obrisi(ctx context.Context, id int64) error
}

// KlijentRepository definiše operacije nad klijentima
type KlijentRepository interface {
	Lista(ctx context.Context, pretraga string) ([]model.Klijent, error)
	DohvatiID(ctx context.Context, id int64) (*model.Klijent, error)
	Kreiraj(ctx context.Context, k *model.Klijent) (int64, error)
	Izmeni(ctx context.Context, k *model.Klijent) error
	Obrisi(ctx context.Context, id int64) error
}

// ServisRepository definiše operacije nad servisnim nalozima
type ServisRepository interface {
	Lista(ctx context.Context, pretraga, status string) ([]model.ServisniNalogSaKlijentom, error)
	DohvatiID(ctx context.Context, id int64) (*model.ServisniNalog, error)
	Kreiraj(ctx context.Context, n *model.ServisniNalog) (int64, error)
	Izmeni(ctx context.Context, n *model.ServisniNalog) error
	Obrisi(ctx context.Context, id int64) error
	SledeciBroj(ctx context.Context) (string, error)
}
