package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

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
		       sd.predlozeno, a.naziv, a.sifra, a.pdv_stopa
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
			&d.Predlozeno, &d.ArtikalNaziv, &d.ArtikalSifra, &d.PdvStopa,
		)
		if err != nil {
			return nil, fmt.Errorf("ntech: ServisniDeloviRepo.DohvatiZaNalog: scan: %w", err)
		}
		d.CenaSaPdv = d.CenaKomada * (1 + d.PdvStopa/100)
		rezultat = append(rezultat, d)
	}

	return rezultat, nil
}

// UgradiIliPotrazuj atomično (jedna transakcija) ugrađuje ono što fizički ima na
// stanju, a višak beleži u potraživane delove. Stanje se čita i koristi unutar iste
// transakcije pa lager NIKAD ne ide u minus (nema TOCTOU između čitanja i upisa).
// Vraća koliko je ugrađeno i koliko nedostaje.
//
// Predloženi delovi (predlozeno=true) ne skidaju sa lagera — sve ide u potraživane
// do odluke klijenta. Tek PrihvatiPredlozene skida sa lagera.
func (r *ServisniDeloviRepo) UgradiIliPotrazuj(ctx context.Context, nalogID, artikalID int64, kolicina int, cenaKomada float64, korisnikID *int64, predlozeno bool) (ugradjeno, nedostaje int, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("ntech: ServisniDeloviRepo.UgradiIliPotrazuj: begin tx: %w", err)
	}
	defer tx.Rollback()

	ugradjeno, nedostaje, err = ugradiIliPotrazujTx(ctx, tx, nalogID, artikalID, kolicina, cenaKomada, korisnikID, predlozeno)
	if err != nil {
		return 0, 0, err
	}

	if err = tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("ntech: ServisniDeloviRepo.UgradiIliPotrazuj: commit: %w", err)
	}
	return ugradjeno, nedostaje, nil
}

// ugradiIliPotrazujTx je interna verzija koja radi unutar postojeće transakcije.
func ugradiIliPotrazujTx(ctx context.Context, tx *sql.Tx, nalogID, artikalID int64, kolicina int, cenaKomada float64, korisnikID *int64, predlozeno bool) (ugradjeno, nedostaje int, err error) {
	// Predloženi delovi: ne skidaju sa lagera, svaki predlog je poseban red (ne merge)
	if predlozeno {
		slog.Info("PREDLOG_INSERT", "nalogID", nalogID, "artikalID", artikalID, "kolicina", kolicina)
		_, err = tx.ExecContext(ctx,
			"INSERT INTO servisni_potrazivani_delovi (nalog_id, artikal_id, kolicina, cena_komada, predlozeno) VALUES (?, ?, ?, ?, 1)",
			nalogID, artikalID, kolicina, cenaKomada,
		)
		if err != nil {
			slog.Error("PREDLOG_INSERT_ERR", "err", err)
			return 0, 0, fmt.Errorf("ntech: ugradiIliPotrazujTx: predlozeni: %w", err)
		}
		slog.Info("PREDLOG_INSERT_OK")
		return 0, kolicina, nil
	}

	// Ugrađeni delovi: standardna logika — skidamo sa lagera
	var stanjePre int
	var nabavnaCena float64
	if err = tx.QueryRowContext(ctx,
		"SELECT kolicina, nabavna_cena FROM artikli WHERE id = ?", artikalID,
	).Scan(&stanjePre, &nabavnaCena); err != nil {
		return 0, 0, fmt.Errorf("ntech: ugradiIliPotrazujTx: dohvati stanje: %w", err)
	}

	ugradjeno = kolicina
	if ugradjeno > stanjePre {
		ugradjeno = stanjePre
	}
	if ugradjeno < 0 {
		ugradjeno = 0
	}
	nedostaje = kolicina - ugradjeno

	if ugradjeno > 0 {
		var postojeciID int64
		var postojeciKol int
		errRed := tx.QueryRowContext(ctx,
			"SELECT id, kolicina FROM servisni_delovi WHERE nalog_id = ? AND artikal_id = ? AND predlozeno = 0",
			nalogID, artikalID,
		).Scan(&postojeciID, &postojeciKol)
		if errRed == nil {
			if _, err = tx.ExecContext(ctx,
				"UPDATE servisni_delovi SET kolicina = ?, cena_komada = ?, nabavna_cena = ? WHERE id = ?",
				postojeciKol+ugradjeno, cenaKomada, nabavnaCena, postojeciID,
			); err != nil {
				return 0, 0, fmt.Errorf("ntech: ugradiIliPotrazujTx: merge: %w", err)
			}
		} else if errors.Is(errRed, sql.ErrNoRows) {
			if _, err = tx.ExecContext(ctx,
				"INSERT INTO servisni_delovi (nalog_id, artikal_id, kolicina, cena_komada, predlozeno, nabavna_cena) VALUES (?, ?, ?, ?, 0, ?)",
				nalogID, artikalID, ugradjeno, cenaKomada, nabavnaCena,
			); err != nil {
				return 0, 0, fmt.Errorf("ntech: ugradiIliPotrazujTx: insert: %w", err)
			}
		} else {
			return 0, 0, fmt.Errorf("ntech: ugradiIliPotrazujTx: proveri: %w", errRed)
		}

		stanjePosle := stanjePre - ugradjeno
		if _, err = tx.ExecContext(ctx,
			"UPDATE artikli SET kolicina = ? WHERE id = ?", stanjePosle, artikalID,
		); err != nil {
			return 0, 0, fmt.Errorf("ntech: ugradiIliPotrazujTx: update stanje: %w", err)
		}
		if err = zabeleziMagacinPromenu(ctx, tx, artikalID, model.PromenaIzlazServis,
			-ugradjeno, stanjePre, stanjePosle, nalogID, korisnikID, ""); err != nil {
			return 0, 0, fmt.Errorf("ntech: ugradiIliPotrazujTx: magacin: %w", err)
		}
	}

	if nedostaje > 0 {
		var postojeciID int64
		var postojeciKol int
		errRed := tx.QueryRowContext(ctx,
			"SELECT id, kolicina FROM servisni_potrazivani_delovi WHERE nalog_id = ? AND artikal_id = ? AND predlozeno = 0",
			nalogID, artikalID,
		).Scan(&postojeciID, &postojeciKol)
		if errRed == nil {
			if _, err = tx.ExecContext(ctx,
				"UPDATE servisni_potrazivani_delovi SET kolicina = ?, cena_komada = ? WHERE id = ?",
				postojeciKol+nedostaje, cenaKomada, postojeciID,
			); err != nil {
				return 0, 0, fmt.Errorf("ntech: ugradiIliPotrazujTx: potraživani update: %w", err)
			}
		} else if errors.Is(errRed, sql.ErrNoRows) {
			if _, err = tx.ExecContext(ctx,
				"INSERT INTO servisni_potrazivani_delovi (nalog_id, artikal_id, kolicina, cena_komada, predlozeno) VALUES (?, ?, ?, ?, 0)",
				nalogID, artikalID, nedostaje, cenaKomada,
			); err != nil {
				return 0, 0, fmt.Errorf("ntech: ugradiIliPotrazujTx: potraživani insert: %w", err)
			}
		} else {
			return 0, 0, fmt.Errorf("ntech: ugradiIliPotrazujTx: potraživani proveri: %w", errRed)
		}
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

// PrihvatiPredlozene prebacuje sve predložene delove (i potraživane) naloga u
// ugrađene (predlozeno → 0) — poziva se kad klijent prihvati predlog popravke.
// Sada zaista skida sa lagera: predloženi delovi prethodno nisu dirali lager.
func (r *ServisniDeloviRepo) PrihvatiPredlozene(ctx context.Context, nalogID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ntech: ServisniDeloviRepo.PrihvatiPredlozene: begin tx: %w", err)
	}
	defer tx.Rollback()

	// Dohvati sve predložene potraživane delove
	redovi, err := tx.QueryContext(ctx,
		"SELECT id, artikal_id, kolicina, cena_komada FROM servisni_potrazivani_delovi WHERE nalog_id = ? AND predlozeno = 1",
		nalogID,
	)
	if err != nil {
		return fmt.Errorf("ntech: ServisniDeloviRepo.PrihvatiPredlozene: dohvati predlozene: %w", err)
	}
	defer redovi.Close()

	type predlozeniStavka struct {
		id         int64
		artikalID  int64
		kolicina   int
		cenaKomada float64
	}
	var stavke []predlozeniStavka
	for redovi.Next() {
		var s predlozeniStavka
		if err := redovi.Scan(&s.id, &s.artikalID, &s.kolicina, &s.cenaKomada); err != nil {
			return fmt.Errorf("ntech: ServisniDeloviRepo.PrihvatiPredlozene: scan: %w", err)
		}
		stavke = append(stavke, s)
	}

	// Za svaki predloženi deo: probaj da ugradiš (skine sa lagera koliko može)
	for _, s := range stavke {
		_, _, err := ugradiIliPotrazujTx(ctx, tx, nalogID, s.artikalID, s.kolicina, s.cenaKomada, nil, false)
		if err != nil {
			return fmt.Errorf("ntech: ServisniDeloviRepo.PrihvatiPredlozene: ugradi: %w", err)
		}
	}

	// Obriši predložene potraživane (prebačeni su u ugrađene/potraživane bez predlozeno)
	if _, err := tx.ExecContext(ctx,
		"DELETE FROM servisni_potrazivani_delovi WHERE nalog_id = ? AND predlozeno = 1", nalogID,
	); err != nil {
		return fmt.Errorf("ntech: ServisniDeloviRepo.PrihvatiPredlozene: obrisi potraživane: %w", err)
	}

	// Očisti i servisni_delovi sa predlozeno=1 (zaostali iz starije verzije)
	if _, err := tx.ExecContext(ctx,
		"DELETE FROM servisni_delovi WHERE nalog_id = ? AND predlozeno = 1", nalogID,
	); err != nil {
		return fmt.Errorf("ntech: ServisniDeloviRepo.PrihvatiPredlozene: obrisi delove: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("ntech: ServisniDeloviRepo.PrihvatiPredlozene: commit: %w", err)
	}
	return nil
}

// ObrisiPredlozene briše sve predložene delove (i potraživane) sa naloga
func (r *ServisniDeloviRepo) ObrisiPredlozene(ctx context.Context, nalogID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ntech: ServisniDeloviRepo.ObrisiPredlozene: begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		"DELETE FROM servisni_delovi WHERE nalog_id = ? AND predlozeno = 1", nalogID,
	); err != nil {
		return fmt.Errorf("ntech: ServisniDeloviRepo.ObrisiPredlozene: delovi: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		"DELETE FROM servisni_potrazivani_delovi WHERE nalog_id = ? AND predlozeno = 1", nalogID,
	); err != nil {
		return fmt.Errorf("ntech: ServisniDeloviRepo.ObrisiPredlozene: potraživani: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("ntech: ServisniDeloviRepo.ObrisiPredlozene: commit: %w", err)
	}
	return nil
}

// PrihvatiOdabranePoArtiklu prihvata selektivno predložene delove: artikalIDs su prihvaćeni
// (idu kroz normalnu ugradnju/lager), svi ostali predlozeni se brišu bez povraćaja lagera.
func (r *ServisniDeloviRepo) PrihvatiOdabranePoArtiklu(ctx context.Context, nalogID int64, artikalIDs []int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ntech: ServisniDeloviRepo.PrihvatiOdabranePoArtiklu: begin tx: %w", err)
	}
	defer tx.Rollback()

	// Dohvati sve predložene potraživane
	redovi, err := tx.QueryContext(ctx,
		"SELECT id, artikal_id, kolicina, cena_komada FROM servisni_potrazivani_delovi WHERE nalog_id = ? AND predlozeno = 1",
		nalogID,
	)
	if err != nil {
		return fmt.Errorf("ntech: ServisniDeloviRepo.PrihvatiOdabranePoArtiklu: dohvati: %w", err)
	}
	type stavka struct {
		id        int64
		artikalID int64
		kolicina  int
		cena      float64
	}
	var sve []stavka
	for redovi.Next() {
		var s stavka
		if err := redovi.Scan(&s.id, &s.artikalID, &s.kolicina, &s.cena); err != nil {
			redovi.Close()
			return fmt.Errorf("ntech: ServisniDeloviRepo.PrihvatiOdabranePoArtiklu: scan: %w", err)
		}
		sve = append(sve, s)
	}
	redovi.Close()
	if err := redovi.Err(); err != nil {
		return fmt.Errorf("ntech: ServisniDeloviRepo.PrihvatiOdabranePoArtiklu: rows: %w", err)
	}

	prihvaceni := make(map[int64]bool, len(artikalIDs))
	for _, id := range artikalIDs {
		prihvaceni[id] = true
	}

	for _, s := range sve {
		if prihvaceni[s.artikalID] {
			if _, _, err := ugradiIliPotrazujTx(ctx, tx, nalogID, s.artikalID, s.kolicina, s.cena, nil, false); err != nil {
				return fmt.Errorf("ntech: ServisniDeloviRepo.PrihvatiOdabranePoArtiklu: ugradi: %w", err)
			}
		}
	}

	// Briši sve predlozene potraživane (prihvaćeni su prebačeni, odbijeni se odbacuju)
	if _, err := tx.ExecContext(ctx,
		"DELETE FROM servisni_potrazivani_delovi WHERE nalog_id = ? AND predlozeno = 1", nalogID,
	); err != nil {
		return fmt.Errorf("ntech: ServisniDeloviRepo.PrihvatiOdabranePoArtiklu: obrisi: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		"DELETE FROM servisni_delovi WHERE nalog_id = ? AND predlozeno = 1", nalogID,
	); err != nil {
		return fmt.Errorf("ntech: ServisniDeloviRepo.PrihvatiOdabranePoArtiklu: obrisi delove: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("ntech: ServisniDeloviRepo.PrihvatiOdabranePoArtiklu: commit: %w", err)
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
