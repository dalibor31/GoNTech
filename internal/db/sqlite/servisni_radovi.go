package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"ntech/internal/model"
)

// ServisniRadoviRepo je SQLite implementacija db.ServisniRadoviRepository
type ServisniRadoviRepo struct {
	db *sql.DB
}

// NoviServisniRadoviRepo kreira repozitorijum radova nad datom bazom
func NoviServisniRadoviRepo(baza *sql.DB) *ServisniRadoviRepo {
	return &ServisniRadoviRepo{db: baza}
}

// DohvatiZaNalog vraća sve radove (usluge) jednog naloga, redom unosa
func (r *ServisniRadoviRepo) DohvatiZaNalog(ctx context.Context, nalogID int64) ([]model.ServisniRad, error) {
	redovi, err := r.db.QueryContext(ctx, `
		SELECT sr.id, sr.nalog_id, COALESCE(sr.usluga_id, 0), sr.naziv, sr.kolicina, sr.cena_komada, sr.datum, sr.predlozeno,
		       COALESCE(u.sifra, '')
		FROM servis_radovi sr
		LEFT JOIN usluge u ON u.id = sr.usluga_id
		WHERE sr.nalog_id = ? ORDER BY sr.id`, nalogID)
	if err != nil {
		return nil, fmt.Errorf("ntech: ServisniRadoviRepo.DohvatiZaNalog: %w", err)
	}
	defer redovi.Close()

	var radovi []model.ServisniRad
	for redovi.Next() {
		var rad model.ServisniRad
		if err := redovi.Scan(&rad.ID, &rad.NalogID, &rad.UslugaID, &rad.Naziv, &rad.Kolicina, &rad.CenaKomada, &rad.Datum, &rad.Predlozeno, &rad.Sifra); err != nil {
			return nil, fmt.Errorf("ntech: ServisniRadoviRepo.DohvatiZaNalog: %w", err)
		}
		radovi = append(radovi, rad)
	}
	return radovi, redovi.Err()
}

// Dodaj upisuje jednu stavku rada na nalog (radovi ne diraju lager).
// predlozeno=true označava uslugu koju serviser predlaže posle dijagnostike.
func (r *ServisniRadoviRepo) Dodaj(ctx context.Context, nalogID, uslugaID int64, naziv string, kolicina, cenaKomada float64, predlozeno bool) (int64, error) {
	var uid any
	if uslugaID > 0 {
		uid = uslugaID
	}
	predInt := 0
	if predlozeno {
		predInt = 1
	}
	rez, err := r.db.ExecContext(ctx, `
		INSERT INTO servis_radovi (nalog_id, usluga_id, naziv, kolicina, cena_komada, predlozeno)
		VALUES (?, ?, ?, ?, ?, ?)`,
		nalogID, uid, naziv, kolicina, cenaKomada, predInt,
	)
	if err != nil {
		return 0, fmt.Errorf("ntech: ServisniRadoviRepo.Dodaj: %w", err)
	}
	id, err := rez.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("ntech: ServisniRadoviRepo.Dodaj: last insert id: %w", err)
	}
	return id, nil
}

// PrihvatiPredlozene prebacuje sve predložene radove naloga u redovne (predlozeno → 0)
func (r *ServisniRadoviRepo) PrihvatiPredlozene(ctx context.Context, nalogID int64) error {
	if _, err := r.db.ExecContext(ctx,
		"UPDATE servis_radovi SET predlozeno = 0 WHERE nalog_id = ? AND predlozeno = 1", nalogID,
	); err != nil {
		return fmt.Errorf("ntech: ServisniRadoviRepo.PrihvatiPredlozene: %w", err)
	}
	return nil
}

// ObrisiPredlozene briše sve predložene radove sa naloga (klijent odbio predlog)
func (r *ServisniRadoviRepo) ObrisiPredlozene(ctx context.Context, nalogID int64) error {
	_, err := r.db.ExecContext(ctx,
		"DELETE FROM servis_radovi WHERE nalog_id = ? AND predlozeno = 1", nalogID,
	)
	if err != nil {
		return fmt.Errorf("ntech: ServisniRadoviRepo.ObrisiPredlozene: %w", err)
	}
	return nil
}

// Obrisi uklanja jednu stavku rada sa naloga
func (r *ServisniRadoviRepo) Obrisi(ctx context.Context, id int64) error {
	if _, err := r.db.ExecContext(ctx, "DELETE FROM servis_radovi WHERE id = ?", id); err != nil {
		return fmt.Errorf("ntech: ServisniRadoviRepo.Obrisi: %w", err)
	}
	return nil
}
