package sqlite

import (
	"context"
	"database/sql"
	"errors"
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

// UgradiIliPotrazuj atomično (jedna transakcija) ugrađuje ono što fizički ima na
// stanju, a višak beleži u potraživane delove. Stanje se čita i koristi unutar iste
// transakcije pa lager NIKAD ne ide u minus (nema TOCTOU između čitanja i upisa).
// Vraća koliko je ugrađeno i koliko nedostaje.
func (r *ServisniDeloviRepo) UgradiIliPotrazuj(ctx context.Context, nalogID, artikalID int64, kolicina int, cenaKomada float64, korisnikID *int64) (ugradjeno, nedostaje int, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("ntech: ServisniDeloviRepo.UgradiIliPotrazuj: begin tx: %w", err)
	}
	defer tx.Rollback()

	var stanjePre int
	if err = tx.QueryRowContext(ctx,
		"SELECT kolicina FROM artikli WHERE id = ?", artikalID,
	).Scan(&stanjePre); err != nil {
		return 0, 0, fmt.Errorf("ntech: ServisniDeloviRepo.UgradiIliPotrazuj: dohvati stanje: %w", err)
	}

	// ugrađujemo najviše onoliko koliko fizički imamo; ostatak ide u potraživane
	ugradjeno = kolicina
	if ugradjeno > stanjePre {
		ugradjeno = stanjePre
	}
	if ugradjeno < 0 {
		ugradjeno = 0
	}
	nedostaje = kolicina - ugradjeno

	// ugradi ono što imamo: merge u servisni_delovi, skini sa magacina, zabeleži promenu
	if ugradjeno > 0 {
		var postojeciID int64
		var postojeciKol int
		errRed := tx.QueryRowContext(ctx,
			"SELECT id, kolicina FROM servisni_delovi WHERE nalog_id = ? AND artikal_id = ?",
			nalogID, artikalID,
		).Scan(&postojeciID, &postojeciKol)
		if errRed == nil {
			if _, err = tx.ExecContext(ctx,
				"UPDATE servisni_delovi SET kolicina = ?, cena_komada = ? WHERE id = ?",
				postojeciKol+ugradjeno, cenaKomada, postojeciID,
			); err != nil {
				return 0, 0, fmt.Errorf("ntech: ServisniDeloviRepo.UgradiIliPotrazuj: merge: %w", err)
			}
		} else if errors.Is(errRed, sql.ErrNoRows) {
			if _, err = tx.ExecContext(ctx, `
				INSERT INTO servisni_delovi (nalog_id, artikal_id, kolicina, cena_komada)
				VALUES (?, ?, ?, ?)`,
				nalogID, artikalID, ugradjeno, cenaKomada,
			); err != nil {
				return 0, 0, fmt.Errorf("ntech: ServisniDeloviRepo.UgradiIliPotrazuj: insert: %w", err)
			}
		} else {
			return 0, 0, fmt.Errorf("ntech: ServisniDeloviRepo.UgradiIliPotrazuj: proveri: %w", errRed)
		}

		stanjePosle := stanjePre - ugradjeno
		if _, err = tx.ExecContext(ctx,
			"UPDATE artikli SET kolicina = ? WHERE id = ?", stanjePosle, artikalID,
		); err != nil {
			return 0, 0, fmt.Errorf("ntech: ServisniDeloviRepo.UgradiIliPotrazuj: update stanje: %w", err)
		}
		if err = zabeleziMagacinPromenu(ctx, tx, artikalID, model.PromenaIzlazServis,
			-ugradjeno, stanjePre, stanjePosle, nalogID, korisnikID, ""); err != nil {
			return 0, 0, fmt.Errorf("ntech: ServisniDeloviRepo.UgradiIliPotrazuj: magacin: %w", err)
		}
	}

	// višak → potraživani delovi (ne skidamo sa lagera dok ne stigne)
	if nedostaje > 0 {
		var postojeciID int64
		var postojeciKol int
		errRed := tx.QueryRowContext(ctx,
			"SELECT id, kolicina FROM servisni_potrazivani_delovi WHERE nalog_id = ? AND artikal_id = ?",
			nalogID, artikalID,
		).Scan(&postojeciID, &postojeciKol)
		if errRed == nil {
			if _, err = tx.ExecContext(ctx,
				"UPDATE servisni_potrazivani_delovi SET kolicina = ?, cena_komada = ? WHERE id = ?",
				postojeciKol+nedostaje, cenaKomada, postojeciID,
			); err != nil {
				return 0, 0, fmt.Errorf("ntech: ServisniDeloviRepo.UgradiIliPotrazuj: potraživani update: %w", err)
			}
		} else if errors.Is(errRed, sql.ErrNoRows) {
			if _, err = tx.ExecContext(ctx,
				"INSERT INTO servisni_potrazivani_delovi (nalog_id, artikal_id, kolicina, cena_komada) VALUES (?, ?, ?, ?)",
				nalogID, artikalID, nedostaje, cenaKomada,
			); err != nil {
				return 0, 0, fmt.Errorf("ntech: ServisniDeloviRepo.UgradiIliPotrazuj: potraživani insert: %w", err)
			}
		} else {
			return 0, 0, fmt.Errorf("ntech: ServisniDeloviRepo.UgradiIliPotrazuj: potraživani proveri: %w", errRed)
		}
	}

	if err = tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("ntech: ServisniDeloviRepo.UgradiIliPotrazuj: commit: %w", err)
	}
	return ugradjeno, nedostaje, nil
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

// DohvatiArtikalID vraća artikal_id za dati servisni deo (pre brisanja)
func (r *ServisniDeloviRepo) DohvatiArtikalID(ctx context.Context, deoID int64) (int64, error) {
	var artikalID int64
	err := r.db.QueryRowContext(ctx,
		"SELECT artikal_id FROM servisni_delovi WHERE id = ?", deoID,
	).Scan(&artikalID)
	if err != nil {
		return 0, fmt.Errorf("ntech: ServisniDeloviRepo.DohvatiArtikalID: %w", err)
	}
	return artikalID, nil
}
