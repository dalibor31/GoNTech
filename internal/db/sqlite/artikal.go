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
			a.id, a.kategorija_id, a.sifra, a.barkod, a.naziv, a.opis,
			a.kolicina, a.kolicina_min, a.lokacija,
			a.nabavna_cena, a.prodajna_cena, a.pdv_stopa, a.marza, a.napomena, a.datum_unosa,
			COALESCE(k.naziv, '') as kategorija_naziv, k.marza as kategorija_marza
		FROM artikli a
		LEFT JOIN kategorije k ON a.kategorija_id = k.id
		WHERE 1=1`

	args := []any{}

	if filter.Pretraga != "" {
		upit += " AND (a.naziv LIKE ? OR a.sifra LIKE ? OR a.barkod LIKE ? OR a.lokacija LIKE ? OR k.naziv LIKE ?)"
		t := "%" + filter.Pretraga + "%"
		args = append(args, t, t, t, t, t)
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
		var sifra, barkod sql.NullString
		var marza, katMarza sql.NullFloat64

		err := redovi.Scan(
			&a.ID, &kategorijaID, &sifra, &barkod, &a.Naziv, &a.Opis,
			&a.Kolicina, &a.KolicinMin, &a.Lokacija,
			&a.NabavnaCena, &a.ProdajnaCena, &a.PdvStopa, &marza, &a.Napomena, &a.DatumUnosa,
			&a.KategorijaNaziv, &katMarza,
		)
		if err != nil {
			return nil, fmt.Errorf("ntech: ArtikalRepo.Lista: scan: %w", err)
		}

		if kategorijaID.Valid {
			a.KategorijaID = &kategorijaID.Int64
		}
		if sifra.Valid {
			a.Sifra = sifra.String
		}
		if barkod.Valid {
			a.Barkod = barkod.String
		}
		if marza.Valid {
			a.Marza = &marza.Float64
		}
		if katMarza.Valid {
			a.KategorijaMarza = &katMarza.Float64
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
	var sifra, barkod sql.NullString
	var marza sql.NullFloat64

	err := r.db.QueryRowContext(ctx, `
		SELECT id, kategorija_id, sifra, barkod, naziv, opis, kolicina, kolicina_min,
		       lokacija, nabavna_cena, prodajna_cena, pdv_stopa, marza, napomena, datum_unosa
		FROM artikli WHERE id = ?`, id).Scan(
		&a.ID, &kategorijaID, &sifra, &barkod, &a.Naziv, &a.Opis,
		&a.Kolicina, &a.KolicinMin, &a.Lokacija,
		&a.NabavnaCena, &a.ProdajnaCena, &a.PdvStopa, &marza, &a.Napomena, &a.DatumUnosa,
	)
	if err != nil {
		return nil, fmt.Errorf("ntech: ArtikalRepo.DohvatiID: %w", err)
	}

	if kategorijaID.Valid {
		a.KategorijaID = &kategorijaID.Int64
	}
	if sifra.Valid {
		a.Sifra = sifra.String
	}
	if barkod.Valid {
		a.Barkod = barkod.String
	}
	if marza.Valid {
		a.Marza = &marza.Float64
	}

	return &a, nil
}

// Kreiraj dodaje novi artikal u bazu
func (r *ArtikalRepo) Kreiraj(ctx context.Context, a *model.Artikal) (int64, error) {
	var sifra, barkod any
	if a.Sifra != "" {
		sifra = a.Sifra
	}
	if a.Barkod != "" {
		barkod = a.Barkod
	}

	rezultat, err := r.db.ExecContext(ctx, `
		INSERT INTO artikli
			(kategorija_id, sifra, barkod, naziv, opis, kolicina, kolicina_min, lokacija,
			 nabavna_cena, prodajna_cena, pdv_stopa, marza, napomena)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.KategorijaID, sifra, barkod, a.Naziv, a.Opis, a.Kolicina, a.KolicinMin,
		a.Lokacija, a.NabavnaCena, a.ProdajnaCena, a.PdvStopa, a.Marza, a.Napomena,
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
	var sifra, barkod any
	if a.Sifra != "" {
		sifra = a.Sifra
	}
	if a.Barkod != "" {
		barkod = a.Barkod
	}

	_, err := r.db.ExecContext(ctx, `
		UPDATE artikli SET
			kategorija_id = ?, sifra = ?, barkod = ?, naziv = ?, opis = ?, kolicina = ?,
			kolicina_min = ?, lokacija = ?,
			nabavna_cena = ?, prodajna_cena = ?, pdv_stopa = ?, marza = ?, napomena = ?
		WHERE id = ?`,
		a.KategorijaID, sifra, barkod, a.Naziv, a.Opis, a.Kolicina,
		a.KolicinMin, a.Lokacija,
		a.NabavnaCena, a.ProdajnaCena, a.PdvStopa, a.Marza, a.Napomena, a.ID,
	)
	if err != nil {
		return fmt.Errorf("ntech: ArtikalRepo.Izmeni: %w", err)
	}

	return nil
}

// SledecaSifra vraća predlog sledeće auto-šifre u formatu ART-XXXXX
func (r *ArtikalRepo) SledecaSifra(ctx context.Context) (string, error) {
	var n int64
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM artikli").Scan(&n)
	if err != nil {
		return "", fmt.Errorf("ntech: ArtikalRepo.SledecaSifra: %w", err)
	}
	return fmt.Sprintf("ART-%05d", n+1), nil
}

// AzurirajCene menja samo nabavnu i prodajnu cenu artikla (kalkulacija pri prijemu robe).
func (r *ArtikalRepo) AzurirajCene(ctx context.Context, id int64, nabavna, prodajna float64) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE artikli SET nabavna_cena = ?, prodajna_cena = ? WHERE id = ?", nabavna, prodajna, id)
	if err != nil {
		return fmt.Errorf("ntech: ArtikalRepo.AzurirajCene: %w", err)
	}
	return nil
}

// PremestiKategoriju menja samo kategoriju artikla (premeštanje u drugu kategoriju).
// kategorijaID može biti nil — tada artikal ostaje bez kategorije.
func (r *ArtikalRepo) PremestiKategoriju(ctx context.Context, id int64, kategorijaID *int64) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE artikli SET kategorija_id = ? WHERE id = ?", kategorijaID, id)
	if err != nil {
		return fmt.Errorf("ntech: ArtikalRepo.PremestiKategoriju: %w", err)
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
