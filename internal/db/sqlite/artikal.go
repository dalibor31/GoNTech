package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"ntech/internal/db"
	"ntech/internal/model"
)

// ArtikalRepo je SQLite implementacija ArtikalRepository interfejsa
type ArtikalRepo struct {
	db *sql.DB
}

// NoviArtikalRepo kreira novi ArtikalRepo
func NoviArtikalRepo(db *sql.DB) *ArtikalRepo {
	return &ArtikalRepo{db: db}
}

// Lista vraća listu artikala sa opcionalnim filterima
func (r *ArtikalRepo) Lista(ctx context.Context, filter db.ArtikalFilter) ([]model.ArtikalSaKategorijom, error) {
	upit := `
		SELECT
			a.id, a.kategorija_id, a.naziv, a.opis,
			a.kolicina, a.kolicina_min, a.lokacija,
			a.nabavna_cena, a.prodajna_cena, a.pdv_stopa, a.napomena, a.datum_unosa,
			COALESCE(k.naziv, '') as kategorija_naziv
		FROM artikli a
		LEFT JOIN kategorije k ON a.kategorija_id = k.id
		WHERE 1=1`

	args := []any{}

	if filter.Pretraga != "" {
		upit += " AND a.naziv LIKE ?"
		args = append(args, "%"+filter.Pretraga+"%")
	}

	if filter.KategorijaID != nil {
		upit += " AND a.kategorija_id = ?"
		args = append(args, *filter.KategorijaID)
	}

	if filter.SamoKriticni {
		upit += " AND a.kolicina <= a.kolicina_min"
	}

	upit += " ORDER BY a.naziv ASC"

	redovi, err := r.db.QueryContext(ctx, upit, args...)
	if err != nil {
		return nil, fmt.Errorf("ntech: ArtikalRepo.Lista: %w", err)
	}
	defer redovi.Close()

	var rezultat []model.ArtikalSaKategorijom
	for redovi.Next() {
		var a model.ArtikalSaKategorijom
		var kategorijaID sql.NullInt64

		err := redovi.Scan(
			&a.ID, &kategorijaID, &a.Naziv, &a.Opis,
			&a.Kolicina, &a.KolicinMin, &a.Lokacija,
			&a.NabavnaCena, &a.ProdajnaCena, &a.PdvStopa, &a.Napomena, &a.DatumUnosa,
			&a.KategorijaNaziv,
		)
		if err != nil {
			return nil, fmt.Errorf("ntech: ArtikalRepo.Lista: scan: %w", err)
		}

		if kategorijaID.Valid {
			a.KategorijaID = &kategorijaID.Int64
		}

		a.KriticnaZaliha = a.Kolicina <= a.KolicinMin

		rezultat = append(rezultat, a)
	}

	return rezultat, nil
}

// DohvatiID vraća jedan artikal po ID-u
func (r *ArtikalRepo) DohvatiID(ctx context.Context, id int64) (*model.Artikal, error) {
	var a model.Artikal
	var kategorijaID sql.NullInt64

	err := r.db.QueryRowContext(ctx, `
		SELECT id, kategorija_id, naziv, opis, kolicina, kolicina_min,
		       lokacija, nabavna_cena, prodajna_cena, pdv_stopa, napomena, datum_unosa
		FROM artikli WHERE id = ?`, id).Scan(
		&a.ID, &kategorijaID, &a.Naziv, &a.Opis,
		&a.Kolicina, &a.KolicinMin, &a.Lokacija,
		&a.NabavnaCena, &a.ProdajnaCena, &a.PdvStopa, &a.Napomena, &a.DatumUnosa,
	)
	if err != nil {
		return nil, fmt.Errorf("ntech: ArtikalRepo.DohvatiID: %w", err)
	}

	if kategorijaID.Valid {
		a.KategorijaID = &kategorijaID.Int64
	}

	return &a, nil
}

// Kreiraj dodaje novi artikal u bazu
func (r *ArtikalRepo) Kreiraj(ctx context.Context, a *model.Artikal) (int64, error) {
	rezultat, err := r.db.ExecContext(ctx, `
		INSERT INTO artikli
			(kategorija_id, naziv, opis, kolicina, kolicina_min, lokacija,
			 nabavna_cena, prodajna_cena, pdv_stopa, napomena)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.KategorijaID, a.Naziv, a.Opis, a.Kolicina, a.KolicinMin,
		a.Lokacija, a.NabavnaCena, a.ProdajnaCena, a.PdvStopa, a.Napomena,
	)
	if err != nil {
		return 0, fmt.Errorf("ntech: ArtikalRepo.Kreiraj: %w", err)
	}

	id, err := rezultat.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("ntech: ArtikalRepo.Kreiraj: last insert id: %w", err)
	}

	return id, nil
}

// Izmeni ažurira postojeći artikal
func (r *ArtikalRepo) Izmeni(ctx context.Context, a *model.Artikal) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE artikli SET
			kategorija_id = ?, naziv = ?, opis = ?, kolicina = ?,
			kolicina_min = ?, lokacija = ?,
			nabavna_cena = ?, prodajna_cena = ?, pdv_stopa = ?, napomena = ?
		WHERE id = ?`,
		a.KategorijaID, a.Naziv, a.Opis, a.Kolicina,
		a.KolicinMin, a.Lokacija,
		a.NabavnaCena, a.ProdajnaCena, a.PdvStopa, a.Napomena, a.ID,
	)
	if err != nil {
		return fmt.Errorf("ntech: ArtikalRepo.Izmeni: %w", err)
	}

	return nil
}

// Obrisi briše artikal po ID-u
func (r *ArtikalRepo) Obrisi(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM artikli WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("ntech: ArtikalRepo.Obrisi: %w", err)
	}

	return nil
}
