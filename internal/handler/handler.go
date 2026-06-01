package handler

import (
	"database/sql"

	"ntech/internal/db"
	"ntech/internal/db/sqlite"
)

// Handler drži zavisnosti koje su potrebne svim handlerima
type Handler struct {
	DB             *sql.DB
	Artikli        db.ArtikalRepository
	KategorijeRepo db.KategorijaRepository
	DobavljaciRepo db.DobavljacRepository
	NabavkeRepo    db.NabavkaRepository
	KlijentiRepo   db.KlijentRepository
}

// Novi kreira novi Handler sa datom bazom
func Novi(baza *sql.DB) *Handler {
	return &Handler{
		DB:             baza,
		Artikli:        sqlite.NoviArtikalRepo(baza),
		KategorijeRepo: sqlite.NovaKategorijaRepo(baza),
		DobavljaciRepo: sqlite.NoviDobavljacRepo(baza),
		NabavkeRepo:    sqlite.NoviNabavkaRepo(baza),
		KlijentiRepo:   sqlite.NoviKlijentRepo(baza),
	}
}
