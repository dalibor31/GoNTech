package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"ntech/internal/model"
)

// ServisniDeloviRepo je SQLite implementacija ServisniDeloviRepository interfejsa
type ServisniDeloviRepo struct {
	db *sql.DB
}

// NoviServisniDeloviRepo kreira novi ServisniDeloviRepo
func NoviServisniDeloviRepo(db *sql.DB) *ServisniDeloviRepo {
	return &ServisniDeloviRepo{db: db}
}

// DohvatiZaNalog vraća sve ugrađene delove za dati servisni nalog
func (r *ServisniDeloviRepo) DohvatiZaNalog(ctx context.Context, nalogID int64) ([]model.ServisniDeoSaArtiklom, error) {
	redovi, err := r.db.QueryContext(ctx, `
		SELECT sd.id, sd.nalog_id, sd.artikal_id, sd.kolicina, sd.cena_komada, sd.datum,
		       a.naziv
		FROM servisni_delovi sd
		JOIN artikli a ON a.id = sd.artikal_id
		WHERE sd.nalog_id = ?
		ORDER BY sd.datum`, nalogID)
	if err != nil {
		return nil, fmt.Errorf("ntech: ServisniDeloviRepo.DohvatiZaNalog: %w", err)
	}
	defer redovi.Close()

	var rezultat []model.ServisniDeoSaArtiklom
	for redovi.Next() {
		var d model.ServisniDeoSaArtiklom
		err := redovi.Scan(
			&d.ID, &d.NalogID, &d.ArtikalID, &d.Kolicina, &d.CenaKomada, &d.Datum,
			&d.ArtikalNaziv,
		)
		if err != nil {
			return nil, fmt.Errorf("ntech: ServisniDeloviRepo.DohvatiZaNalog: scan: %w", err)
		}
		rezultat = append(rezultat, d)
	}

	return rezultat, nil
}

// Dodaj dodaje jedan artikal u servisni nalog, smanjuje stanje u magacinu i beleži promenu
func (r *ServisniDeloviRepo) Dodaj(ctx context.Context, nalogID, artikalID int64, kolicina int, cenaKomada float64, korisnikID *int64) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("ntech: ServisniDeloviRepo.Dodaj: begin tx: %w", err)
	}
	defer tx.Rollback()

	var naziv string
	var stanjePre int
	err = tx.QueryRowContext(ctx,
		"SELECT naziv, kolicina FROM artikli WHERE id = ?", artikalID,
	).Scan(&naziv, &stanjePre)
	if err != nil {
		return 0, fmt.Errorf("ntech: ServisniDeloviRepo.Dodaj: dohvati artikal: %w", err)
	}

	// dozvoljavamo backorder: deo se može ugraditi i kad nema dovoljno na stanju
	// (lager tada ide u minus kao signal da treba nabavka); handler nalog prebacuje
	// u „Čeka delove". Zato ovde NE odbijamo prekoračenje.
	stanjePosle := stanjePre - kolicina
	_, err = tx.ExecContext(ctx,
		"UPDATE artikli SET kolicina = ? WHERE id = ?", stanjePosle, artikalID,
	)
	if err != nil {
		return 0, fmt.Errorf("ntech: ServisniDeloviRepo.Dodaj: update stanje: %w", err)
	}

	rezultat, err := tx.ExecContext(ctx, `
		INSERT INTO servisni_delovi (nalog_id, artikal_id, kolicina, cena_komada)
		VALUES (?, ?, ?, ?)`,
		nalogID, artikalID, kolicina, cenaKomada,
	)
	if err != nil {
		return 0, fmt.Errorf("ntech: ServisniDeloviRepo.Dodaj: insert: %w", err)
	}

	deoID, err := rezultat.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("ntech: ServisniDeloviRepo.Dodaj: last insert id: %w", err)
	}

	err = zabeleziMagacinPromenu(ctx, tx, artikalID, model.PromenaIzlazServis,
		-kolicina, stanjePre, stanjePosle, nalogID, korisnikID, "")
	if err != nil {
		return 0, fmt.Errorf("ntech: ServisniDeloviRepo.Dodaj: magacin: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("ntech: ServisniDeloviRepo.Dodaj: commit: %w", err)
	}

	return deoID, nil
}

// Obrisi uklanja servisni deo i vraća količinu na stanje u magacinu
func (r *ServisniDeloviRepo) Obrisi(ctx context.Context, id int64, korisnikID *int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ntech: ServisniDeloviRepo.Obrisi: begin tx: %w", err)
	}
	defer tx.Rollback()

	var artikalID int64
	var nalogID int64
	var kolicina int
	err = tx.QueryRowContext(ctx,
		"SELECT artikal_id, nalog_id, kolicina FROM servisni_delovi WHERE id = ?", id,
	).Scan(&artikalID, &nalogID, &kolicina)
	if err != nil {
		return fmt.Errorf("ntech: ServisniDeloviRepo.Obrisi: dohvati deo: %w", err)
	}

	var stanjePre int
	err = tx.QueryRowContext(ctx,
		"SELECT kolicina FROM artikli WHERE id = ?", artikalID,
	).Scan(&stanjePre)
	if err != nil {
		return fmt.Errorf("ntech: ServisniDeloviRepo.Obrisi: dohvati stanje: %w", err)
	}

	stanjePosle := stanjePre + kolicina
	_, err = tx.ExecContext(ctx,
		"UPDATE artikli SET kolicina = ? WHERE id = ?", stanjePosle, artikalID,
	)
	if err != nil {
		return fmt.Errorf("ntech: ServisniDeloviRepo.Obrisi: vrati stanje: %w", err)
	}

	_, err = tx.ExecContext(ctx, "DELETE FROM servisni_delovi WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("ntech: ServisniDeloviRepo.Obrisi: delete: %w", err)
	}

	err = zabeleziMagacinPromenu(ctx, tx, artikalID, model.PromenaPovracaj,
		kolicina, stanjePre, stanjePosle, nalogID, korisnikID, "uklonjen servisni deo")
	if err != nil {
		return fmt.Errorf("ntech: ServisniDeloviRepo.Obrisi: magacin: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("ntech: ServisniDeloviRepo.Obrisi: commit: %w", err)
	}

	return nil
}
