package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"ntech/internal/model"
)

// KpoRepo je SQLite implementacija KpoRepository interfejsa.
type KpoRepo struct {
	db *sql.DB
}

// NoviKpoRepo kreira novi KpoRepo.
func NoviKpoRepo(db *sql.DB) *KpoRepo {
	return &KpoRepo{db: db}
}

// Lista vraća KPO unose u zadatom periodu (po datumu prometa); nulti datum = bez granice.
func (r *KpoRepo) Lista(ctx context.Context, od, do time.Time) ([]model.KpoZapis, error) {
	q := `SELECT id, datum_prometa, redni_broj, broj_dokumenta, opis, prihod,
	             nacin_placanja, napomena, izvor, izvor_id, datum_unosa
	      FROM kpo_unosi WHERE 1=1`
	var args []any
	if !od.IsZero() {
		q += " AND datum_prometa >= ?"
		args = append(args, od.Format("2006-01-02"))
	}
	if !do.IsZero() {
		q += " AND datum_prometa <= ?"
		args = append(args, do.Format("2006-01-02"))
	}
	q += " ORDER BY datum_prometa, id"

	redovi, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("ntech: KpoRepo.Lista: %w", err)
	}
	defer redovi.Close()

	var rezultat []model.KpoZapis
	for redovi.Next() {
		var z model.KpoZapis
		var opis, nacinPlacanja, napomena sql.NullString
		var redniBroj sql.NullInt64
		var izvorID sql.NullInt64

		err := redovi.Scan(
			&z.ID, &z.DatumPrometa, &redniBroj, &z.BrojDokumenta, &opis, &z.Prihod,
			&nacinPlacanja, &napomena, &z.Izvor, &izvorID, &z.DatumUnosa,
		)
		if err != nil {
			return nil, fmt.Errorf("ntech: KpoRepo.Lista: scan: %w", err)
		}
		if redniBroj.Valid {
			n := int(redniBroj.Int64)
			z.RedniBroj = &n
		}
		z.Opis = opis.String
		z.NacinPlacanja = nacinPlacanja.String
		z.Napomena = napomena.String
		if izvorID.Valid {
			z.IzvorID = &izvorID.Int64
		}
		rezultat = append(rezultat, z)
	}
	return rezultat, redovi.Err()
}

// Kreiraj upisuje novi KPO zapis i vraća njegov ID. redni_broj se dodeljuje
// automatski kao sledeći u nizu (Pravilnik o KPO zahteva kontinuiran redni broj
// bez prekida — knjiga se nikad ne briše, pa brojevi ostaju u nizu i posle storna).
func (r *KpoRepo) Kreiraj(ctx context.Context, z *model.KpoZapis) (int64, error) {
	rez, err := r.db.ExecContext(ctx, `
		INSERT INTO kpo_unosi
			(datum_prometa, redni_broj, broj_dokumenta, opis, prihod, nacin_placanja, napomena, izvor, izvor_id)
		VALUES (?, (SELECT COALESCE(MAX(redni_broj), 0) + 1 FROM kpo_unosi), ?, ?, ?, ?, ?, ?, ?)`,
		z.DatumPrometa.Format("2006-01-02"),
		z.BrojDokumenta,
		nullString(z.Opis),
		z.Prihod,
		nullString(z.NacinPlacanja),
		nullString(z.Napomena),
		izvorIliRucno(z.Izvor),
		izvorIDArg(z.IzvorID),
	)
	if err != nil {
		return 0, fmt.Errorf("ntech: KpoRepo.Kreiraj: %w", err)
	}
	id, err := rez.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("ntech: KpoRepo.Kreiraj: id: %w", err)
	}
	return id, nil
}

// Obrisi briše KPO zapis po ID-ju.
func (r *KpoRepo) Obrisi(ctx context.Context, id int64) error {
	if _, err := r.db.ExecContext(ctx, "DELETE FROM kpo_unosi WHERE id = ?", id); err != nil {
		return fmt.Errorf("ntech: KpoRepo.Obrisi: %w", err)
	}
	return nil
}

// PostojiZaIzvor vraća true ako postoji bar jedan KPO zapis za dati izvor i izvorID.
func (r *KpoRepo) PostojiZaIzvor(ctx context.Context, izvor string, izvorID int64) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM kpo_unosi WHERE izvor = ? AND izvor_id = ?", izvor, izvorID,
	).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("ntech: KpoRepo.PostojiZaIzvor: %w", err)
	}
	return n > 0, nil
}

// DohvatiPoIzvoru vraća sve KPO zapise vezane za dati izvor (npr. za storno prodaje).
func (r *KpoRepo) DohvatiPoIzvoru(ctx context.Context, izvor string, izvorID int64) ([]model.KpoZapis, error) {
	redovi, err := r.db.QueryContext(ctx, `
		SELECT id, datum_prometa, redni_broj, broj_dokumenta, opis, prihod,
		       nacin_placanja, napomena, izvor, izvor_id, datum_unosa
		FROM kpo_unosi WHERE izvor = ? AND izvor_id = ?`, izvor, izvorID)
	if err != nil {
		return nil, fmt.Errorf("ntech: KpoRepo.DohvatiPoIzvoru: %w", err)
	}
	defer redovi.Close()

	var rezultat []model.KpoZapis
	for redovi.Next() {
		var z model.KpoZapis
		var opis, nacinPlacanja, napomena sql.NullString
		var redniBroj sql.NullInt64
		var izvorIDCol sql.NullInt64

		if err := redovi.Scan(
			&z.ID, &z.DatumPrometa, &redniBroj, &z.BrojDokumenta, &opis, &z.Prihod,
			&nacinPlacanja, &napomena, &z.Izvor, &izvorIDCol, &z.DatumUnosa,
		); err != nil {
			return nil, fmt.Errorf("ntech: KpoRepo.DohvatiPoIzvoru: scan: %w", err)
		}
		if redniBroj.Valid {
			n := int(redniBroj.Int64)
			z.RedniBroj = &n
		}
		z.Opis = opis.String
		z.NacinPlacanja = nacinPlacanja.String
		z.Napomena = napomena.String
		if izvorIDCol.Valid {
			z.IzvorID = &izvorIDCol.Int64
		}
		rezultat = append(rezultat, z)
	}
	return rezultat, redovi.Err()
}
