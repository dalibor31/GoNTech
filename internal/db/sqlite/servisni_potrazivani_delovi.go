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

// ProveriIPocistiZaArtikal poziva se nakon što stanje artikla poraste (nabavka
// ili direktan unos na kartici/popisu) i čisti potraživane redove koji se sada
// mogu pokriti dostupnim stanjem (FIFO).
//
// Kada se potraženi deo (delimično ili u celosti) pokrije, ta količina se ODMAH
// skida sa magacina jer fizički odlazi na servisni nalog; svaka takva promena se
// beleži u magacinski revizijski trag (tip PromenaIzlazServis). Delimično
// pokrivanje smanjuje traženu količinu umesto brisanja reda.
//
// Sve se izvršava u jednoj transakciji da bi skidanje magacina i čišćenje
// potraživanih redova bili atomični.
//
// Vraća ID-eve naloga čiji su svi potraživani redovi obrisani (nalog može da se otključa).
func (r *ServisniPotrazivaniDeloviRepo) ProveriIPocistiZaArtikal(ctx context.Context, artikalID int64) ([]int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("ntech: ServisniPotrazivaniDeloviRepo.ProveriIPocistiZaArtikal: begin: %w", err)
	}
	defer tx.Rollback()

	var stanje int
	err = tx.QueryRowContext(ctx, "SELECT kolicina FROM artikli WHERE id = ?", artikalID).Scan(&stanje)
	if err != nil {
		return nil, fmt.Errorf("ntech: ServisniPotrazivaniDeloviRepo.ProveriIPocistiZaArtikal: stanje: %w", err)
	}
	if stanje <= 0 {
		return nil, nil
	}

	redovi, err := tx.QueryContext(ctx,
		"SELECT id, nalog_id, kolicina FROM servisni_potrazivani_delovi WHERE artikal_id = ? ORDER BY datum",
		artikalID,
	)
	if err != nil {
		return nil, fmt.Errorf("ntech: ServisniPotrazivaniDeloviRepo.ProveriIPocistiZaArtikal: query: %w", err)
	}

	type red struct {
		id       int64
		nalogID  int64
		kolicina int
	}
	var lista []red
	for redovi.Next() {
		var p red
		if err := redovi.Scan(&p.id, &p.nalogID, &p.kolicina); err != nil {
			redovi.Close()
			return nil, fmt.Errorf("ntech: ServisniPotrazivaniDeloviRepo.ProveriIPocistiZaArtikal: scan: %w", err)
		}
		lista = append(lista, p)
	}
	redovi.Close()

	// prati naloge iz kojih smo obrisali red
	obrisaniNalozi := map[int64]struct{}{}

	// tekuće stanje magacina koje se umanjuje kako pokrivamo potraživane delove;
	// svaka promena se beleži sa stanjePre/stanjePosle radi tačnog revizijskog traga
	stanjeMagacin := stanje
	dostupno := stanje
	for _, p := range lista {
		if dostupno <= 0 {
			break
		}
		if dostupno >= p.kolicina {
			// ceo red je pokriven — obriši ga i skini punu količinu sa magacina
			if _, err := tx.ExecContext(ctx, "DELETE FROM servisni_potrazivani_delovi WHERE id = ?", p.id); err != nil {
				return nil, fmt.Errorf("ntech: ServisniPotrazivaniDeloviRepo.ProveriIPocistiZaArtikal: delete: %w", err)
			}
			if err := r.skiniSaMagacina(ctx, tx, artikalID, p.kolicina, &stanjeMagacin, p.nalogID); err != nil {
				return nil, err
			}
			obrisaniNalozi[p.nalogID] = struct{}{}
			dostupno -= p.kolicina
		} else {
			// delimično pokrivanje — sve dostupno ide na nalog; skini dostupno sa magacina
			novaKol := p.kolicina - dostupno
			if _, err := tx.ExecContext(ctx, "UPDATE servisni_potrazivani_delovi SET kolicina = ? WHERE id = ?", novaKol, p.id); err != nil {
				return nil, fmt.Errorf("ntech: ServisniPotrazivaniDeloviRepo.ProveriIPocistiZaArtikal: update: %w", err)
			}
			if err := r.skiniSaMagacina(ctx, tx, artikalID, dostupno, &stanjeMagacin, p.nalogID); err != nil {
				return nil, err
			}
			dostupno = 0
		}
	}

	// vrati samo one naloge koji NEMAJU više nijedan potraživani red
	var otkljucani []int64
	for nalogID := range obrisaniNalozi {
		var preostalo int
		if err := tx.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM servisni_potrazivani_delovi WHERE nalog_id = ?", nalogID,
		).Scan(&preostalo); err != nil {
			return nil, fmt.Errorf("ntech: ServisniPotrazivaniDeloviRepo.ProveriIPocistiZaArtikal: prebroji: %w", err)
		}
		if preostalo == 0 {
			otkljucani = append(otkljucani, nalogID)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("ntech: ServisniPotrazivaniDeloviRepo.ProveriIPocistiZaArtikal: commit: %w", err)
	}
	return otkljucani, nil
}

// skiniSaMagacina umanjuje stanje artikla za datu količinu (jer odlazi na servisni
// nalog) i upisuje promenu u magacinski trag tipa PromenaIzlazServis. Radi unutar
// prosleđene transakcije; *stanje drži tekuće stanje radi tačnog stanjePre/stanjePosle.
func (r *ServisniPotrazivaniDeloviRepo) skiniSaMagacina(ctx context.Context, tx *sql.Tx, artikalID int64, kolicina int, stanje *int, nalogID int64) error {
	stanjePre := *stanje
	stanjePosle := stanjePre - kolicina
	if _, err := tx.ExecContext(ctx, "UPDATE artikli SET kolicina = ? WHERE id = ?", stanjePosle, artikalID); err != nil {
		return fmt.Errorf("ntech: ServisniPotrazivaniDeloviRepo.skiniSaMagacina: update stanje: %w", err)
	}
	if err := zabeleziMagacinPromenu(ctx, tx, artikalID, model.PromenaIzlazServis,
		-kolicina, stanjePre, stanjePosle, nalogID, nil, "pokriven potraživani deo"); err != nil {
		return fmt.Errorf("ntech: ServisniPotrazivaniDeloviRepo.skiniSaMagacina: magacin: %w", err)
	}
	*stanje = stanjePosle
	return nil
}
