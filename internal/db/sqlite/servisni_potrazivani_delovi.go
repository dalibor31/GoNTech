package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"ntech/internal/model"
)

// ServisniPotrazivaniDeloviRepo je SQLite implementacija ServisniPotrazivaniDeloviRepository
type ServisniPotrazivaniDeloviRepo struct {
	db *sql.DB
}

// NoviServisniPotrazivaniDeloviRepo kreira novi repo
func NoviServisniPotrazivaniDeloviRepo(db *sql.DB) *ServisniPotrazivaniDeloviRepo {
	return &ServisniPotrazivaniDeloviRepo{db: db}
}

// DohvatiZaNalog vraća sve potraživane delove za dati servisni nalog
func (r *ServisniPotrazivaniDeloviRepo) DohvatiZaNalog(ctx context.Context, nalogID int64) ([]model.ServisniPotrazivaniDeo, error) {
	redovi, err := r.db.QueryContext(ctx, `
		SELECT spd.id, spd.nalog_id, spd.artikal_id, spd.kolicina, spd.datum,
		       a.naziv
		FROM servisni_potrazivani_delovi spd
		JOIN artikli a ON a.id = spd.artikal_id
		WHERE spd.nalog_id = ?
		ORDER BY spd.datum`, nalogID)
	if err != nil {
		return nil, fmt.Errorf("ntech: ServisniPotrazivaniDeloviRepo.DohvatiZaNalog: %w", err)
	}
	defer redovi.Close()

	var rezultat []model.ServisniPotrazivaniDeo
	for redovi.Next() {
		var d model.ServisniPotrazivaniDeo
		err := redovi.Scan(&d.ID, &d.NalogID, &d.ArtikalID, &d.Kolicina, &d.Datum, &d.ArtikalNaziv)
		if err != nil {
			return nil, fmt.Errorf("ntech: ServisniPotrazivaniDeloviRepo.DohvatiZaNalog: scan: %w", err)
		}
		rezultat = append(rezultat, d)
	}
	return rezultat, nil
}

// DodajIliUvecaj dodaje novi potraživani deo ili uvećava količinu ako već postoji.
// Vraća ID reda (novog ili postojećeg).
func (r *ServisniPotrazivaniDeloviRepo) DodajIliUvecaj(ctx context.Context, nalogID, artikalID int64, kolicina int) (int64, error) {
	var postojeciID int64
	var postojeciKol int
	err := r.db.QueryRowContext(ctx,
		"SELECT id, kolicina FROM servisni_potrazivani_delovi WHERE nalog_id = ? AND artikal_id = ?",
		nalogID, artikalID,
	).Scan(&postojeciID, &postojeciKol)

	if err == nil {
		novaKol := postojeciKol + kolicina
		_, err = r.db.ExecContext(ctx,
			"UPDATE servisni_potrazivani_delovi SET kolicina = ? WHERE id = ?",
			novaKol, postojeciID,
		)
		if err != nil {
			return 0, fmt.Errorf("ntech: ServisniPotrazivaniDeloviRepo.DodajIliUvecaj: update: %w", err)
		}
		return postojeciID, nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		rez, err := r.db.ExecContext(ctx,
			"INSERT INTO servisni_potrazivani_delovi (nalog_id, artikal_id, kolicina) VALUES (?, ?, ?)",
			nalogID, artikalID, kolicina,
		)
		if err != nil {
			return 0, fmt.Errorf("ntech: ServisniPotrazivaniDeloviRepo.DodajIliUvecaj: insert: %w", err)
		}
		id, err := rez.LastInsertId()
		if err != nil {
			return 0, fmt.Errorf("ntech: ServisniPotrazivaniDeloviRepo.DodajIliUvecaj: last insert id: %w", err)
		}
		return id, nil
	}

	return 0, fmt.Errorf("ntech: ServisniPotrazivaniDeloviRepo.DodajIliUvecaj: proveri: %w", err)
}

// Obrisi uklanja potraživani deo po ID-u
func (r *ServisniPotrazivaniDeloviRepo) Obrisi(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM servisni_potrazivani_delovi WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("ntech: ServisniPotrazivaniDeloviRepo.Obrisi: %w", err)
	}
	return nil
}

// ObrisiZaArtikal briše sve potraživane delove za dati artikal na datom nalogu
func (r *ServisniPotrazivaniDeloviRepo) ObrisiZaArtikal(ctx context.Context, nalogID, artikalID int64) error {
	_, err := r.db.ExecContext(ctx,
		"DELETE FROM servisni_potrazivani_delovi WHERE nalog_id = ? AND artikal_id = ?",
		nalogID, artikalID,
	)
	if err != nil {
		return fmt.Errorf("ntech: ServisniPotrazivaniDeloviRepo.ObrisiZaArtikal: %w", err)
	}
	return nil
}
