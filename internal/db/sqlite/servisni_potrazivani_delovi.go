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
		SELECT spd.id, spd.nalog_id, spd.artikal_id, spd.kolicina, spd.cena_komada, spd.datum,
		       spd.predlozeno, a.naziv
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
		err := redovi.Scan(&d.ID, &d.NalogID, &d.ArtikalID, &d.Kolicina, &d.CenaKomada, &d.Datum, &d.Predlozeno, &d.ArtikalNaziv)
		if err != nil {
			return nil, fmt.Errorf("ntech: ServisniPotrazivaniDeloviRepo.DohvatiZaNalog: scan: %w", err)
		}
		rezultat = append(rezultat, d)
	}
	return rezultat, nil
}

// Obrisi uklanja potraživani deo po ID-u
func (r *ServisniPotrazivaniDeloviRepo) Obrisi(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM servisni_potrazivani_delovi WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("ntech: ServisniPotrazivaniDeloviRepo.Obrisi: %w", err)
	}
	return nil
}

// ObrisiPredlozeneZaArtikal briše samo predlozene redove (predlozeno=1) za dati artikal na nalogu
func (r *ServisniPotrazivaniDeloviRepo) ObrisiPredlozeneZaArtikal(ctx context.Context, nalogID, artikalID int64) error {
	_, err := r.db.ExecContext(ctx,
		"DELETE FROM servisni_potrazivani_delovi WHERE nalog_id = ? AND artikal_id = ? AND predlozeno = 1",
		nalogID, artikalID,
	)
	if err != nil {
		return fmt.Errorf("ntech: ServisniPotrazivaniDeloviRepo.ObrisiPredlozeneZaArtikal: %w", err)
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
		"SELECT id, nalog_id, kolicina, cena_komada FROM servisni_potrazivani_delovi WHERE artikal_id = ? AND predlozeno = 0 ORDER BY datum",
		artikalID,
	)
	if err != nil {
		return nil, fmt.Errorf("ntech: ServisniPotrazivaniDeloviRepo.ProveriIPocistiZaArtikal: query: %w", err)
	}

	type red struct {
		id       int64
		nalogID  int64
		kolicina int
		cena     float64
	}
	var lista []red
	for redovi.Next() {
		var p red
		if err := redovi.Scan(&p.id, &p.nalogID, &p.kolicina, &p.cena); err != nil {
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
			// ceo red je pokriven — prebaci ga u ugrađene delove, skini sa magacina, obriši red
			if err := r.ugradiUNalog(ctx, tx, p.nalogID, artikalID, p.kolicina, p.cena); err != nil {
				return nil, err
			}
			if err := r.skiniSaMagacina(ctx, tx, artikalID, p.kolicina, &stanjeMagacin, p.nalogID); err != nil {
				return nil, err
			}
			if _, err := tx.ExecContext(ctx, "DELETE FROM servisni_potrazivani_delovi WHERE id = ?", p.id); err != nil {
				return nil, fmt.Errorf("ntech: ServisniPotrazivaniDeloviRepo.ProveriIPocistiZaArtikal: delete: %w", err)
			}
			obrisaniNalozi[p.nalogID] = struct{}{}
			dostupno -= p.kolicina
		} else {
			// delimično pokrivanje — sve dostupno ide na nalog; prebaci u ugrađene, skini sa magacina
			if err := r.ugradiUNalog(ctx, tx, p.nalogID, artikalID, dostupno, p.cena); err != nil {
				return nil, err
			}
			if err := r.skiniSaMagacina(ctx, tx, artikalID, dostupno, &stanjeMagacin, p.nalogID); err != nil {
				return nil, err
			}
			novaKol := p.kolicina - dostupno
			if _, err := tx.ExecContext(ctx, "UPDATE servisni_potrazivani_delovi SET kolicina = ? WHERE id = ?", novaKol, p.id); err != nil {
				return nil, fmt.Errorf("ntech: ServisniPotrazivaniDeloviRepo.ProveriIPocistiZaArtikal: update: %w", err)
			}
			dostupno = 0
		}
	}

	// vrati samo one naloge koji NEMAJU više nijedan potraživani red
	var otkljucani []int64
	for nalogID := range obrisaniNalozi {
		var preostalo int
		if err := tx.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM servisni_potrazivani_delovi WHERE nalog_id = ? AND predlozeno = 0", nalogID,
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

// ugradiUNalog prebacuje pokrivenu količinu u ugrađene delove (servisni_delovi):
// uvećava postojeći red za isti artikal na nalogu ili kreira novi. Radi unutar
// prosleđene transakcije. Ne dira magacin — to radi skiniSaMagacina zasebno.
func (r *ServisniPotrazivaniDeloviRepo) ugradiUNalog(ctx context.Context, tx *sql.Tx, nalogID, artikalID int64, kolicina int, cenaKomada float64) error {
	var postojeciID int64
	var postojeciKol int
	err := tx.QueryRowContext(ctx,
		"SELECT id, kolicina FROM servisni_delovi WHERE nalog_id = ? AND artikal_id = ? AND predlozeno = 0",
		nalogID, artikalID,
	).Scan(&postojeciID, &postojeciKol)

	if err == nil {
		// ne pregazi postojeću cenu nulom (npr. legacy potraživani red bez cene)
		if cenaKomada > 0 {
			_, err = tx.ExecContext(ctx,
				"UPDATE servisni_delovi SET kolicina = ?, cena_komada = ? WHERE id = ?",
				postojeciKol+kolicina, cenaKomada, postojeciID,
			)
		} else {
			_, err = tx.ExecContext(ctx,
				"UPDATE servisni_delovi SET kolicina = ? WHERE id = ?",
				postojeciKol+kolicina, postojeciID,
			)
		}
		if err != nil {
			return fmt.Errorf("ntech: ServisniPotrazivaniDeloviRepo.ugradiUNalog: update: %w", err)
		}
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO servisni_delovi (nalog_id, artikal_id, kolicina, cena_komada)
			VALUES (?, ?, ?, ?)`,
			nalogID, artikalID, kolicina, cenaKomada,
		)
		if err != nil {
			return fmt.Errorf("ntech: ServisniPotrazivaniDeloviRepo.ugradiUNalog: insert: %w", err)
		}
		return nil
	}
	return fmt.Errorf("ntech: ServisniPotrazivaniDeloviRepo.ugradiUNalog: proveri: %w", err)
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
