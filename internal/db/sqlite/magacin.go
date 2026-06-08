package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"ntech/internal/model"
)

// MagacinskePromeneRepo je SQLite implementacija MagacinskePromeneRepository interfejsa
type MagacinskePromeneRepo struct {
	db *sql.DB
}

// NoviMagacinskePromeneRepo kreira novi MagacinskePromeneRepo
func NoviMagacinskePromeneRepo(db *sql.DB) *MagacinskePromeneRepo {
	return &MagacinskePromeneRepo{db: db}
}

// Lista vraća listu magacinskih promena, opcionalno filtrirano po artiklu
func (r *MagacinskePromeneRepo) Lista(ctx context.Context, artikalID *int64, limit int) ([]model.MagacinskaPromenaSaDetaljem, error) {
	if limit <= 0 {
		limit = 100
	}

	upit := `
		SELECT mp.id, mp.artikal_id, a.naziv, mp.tip_promene, mp.referentni_id,
		       mp.promena_kolicine, mp.stanje_pre, mp.stanje_posle,
		       mp.korisnik_id, mp.napomena, mp.datum
		FROM magacinske_promene mp
		JOIN artikli a ON a.id = mp.artikal_id
		WHERE 1=1`

	args := []any{}

	if artikalID != nil {
		upit += " AND mp.artikal_id = ?"
		args = append(args, *artikalID)
	}

	upit += " ORDER BY mp.datum DESC LIMIT ?"
	args = append(args, limit)

	redovi, err := r.db.QueryContext(ctx, upit, args...)
	if err != nil {
		return nil, fmt.Errorf("ntech: MagacinskePromeneRepo.Lista: %w", err)
	}
	defer redovi.Close()

	var rezultat []model.MagacinskaPromenaSaDetaljem
	for redovi.Next() {
		var p model.MagacinskaPromenaSaDetaljem
		var korisnikID sql.NullInt64
		var napomena sql.NullString

		err := redovi.Scan(
			&p.ID, &p.ArtikalID, &p.ArtikalNaziv, &p.TipPromene, &p.ReferentniID,
			&p.PromenaKolicine, &p.StanjePre, &p.StanjePosle,
			&korisnikID, &napomena, &p.Datum,
		)
		if err != nil {
			return nil, fmt.Errorf("ntech: MagacinskePromeneRepo.Lista: scan: %w", err)
		}

		if korisnikID.Valid {
			v := korisnikID.Int64
			p.KorisnikID = &v
		}
		p.Napomena = napomena.String

		rezultat = append(rezultat, p)
	}

	return rezultat, nil
}

// zabeleziMagacinPromenu je interni helper koji upisuje jednu promenu stanja artikla.
// Poziva se iz Kreiraj/Storno unutar postojeće transakcije.
func zabeleziMagacinPromenu(
	ctx context.Context,
	tx *sql.Tx,
	artikalID int64,
	tipPromene string,
	promenaKolicine, stanjePre, stanjePosle int,
	referentniID int64,
	korisnikID *int64,
	napomena string,
) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO magacinske_promene
			(artikal_id, tip_promene, promena_kolicine, stanje_pre, stanje_posle,
			 referentni_id, korisnik_id, napomena)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		artikalID, tipPromene, promenaKolicine, stanjePre, stanjePosle,
		referentniID, nullInt64(korisnikID), nullString(napomena),
	)
	if err != nil {
		return fmt.Errorf("ntech: zabeleziMagacinPromenu: %w", err)
	}
	return nil
}
