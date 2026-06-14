package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"ntech/internal/model"
)

// NabavkaRepo je SQLite implementacija NabavkaRepository interfejsa
type NabavkaRepo struct {
	db *sql.DB
}

// NoviNabavkaRepo kreira novi NabavkaRepo
func NoviNabavkaRepo(db *sql.DB) *NabavkaRepo {
	return &NabavkaRepo{db: db}
}

// Lista vraća sve nabavke sa nazivom dobavljača, sortirano od najnovije
func (r *NabavkaRepo) Lista(ctx context.Context) ([]model.NabavkaSaDetaljem, error) {
	redovi, err := r.db.QueryContext(ctx, `
		SELECT
			n.id, n.dobavljac_id, n.napomena, n.ukupno, n.metod_raspodele, n.datum,
			COALESCE(d.naziv, '') AS dobavljac_naziv
		FROM nabavke n
		LEFT JOIN dobavljaci d ON n.dobavljac_id = d.id
		ORDER BY n.datum DESC`)
	if err != nil {
		return nil, fmt.Errorf("ntech: NabavkaRepo.Lista: %w", err)
	}
	defer redovi.Close()

	var rezultat []model.NabavkaSaDetaljem
	for redovi.Next() {
		var n model.NabavkaSaDetaljem
		var dobavljacID sql.NullInt64
		var napomena, metod sql.NullString

		err := redovi.Scan(
			&n.ID, &dobavljacID, &napomena, &n.Ukupno, &metod, &n.Datum,
			&n.DobavljacNaziv,
		)
		if err != nil {
			return nil, fmt.Errorf("ntech: NabavkaRepo.Lista: scan: %w", err)
		}

		if dobavljacID.Valid {
			n.DobavljacID = &dobavljacID.Int64
		}
		n.Napomena = napomena.String
		n.MetodRaspodele = metod.String

		rezultat = append(rezultat, n)
	}

	return rezultat, nil
}

// DohvatiID vraća zaglavlje jedne nabavke po ID-u
func (r *NabavkaRepo) DohvatiID(ctx context.Context, id int64) (*model.Nabavka, error) {
	var n model.Nabavka
	var dobavljacID sql.NullInt64
	var napomena, metod sql.NullString

	err := r.db.QueryRowContext(ctx, `
		SELECT id, dobavljac_id, napomena, ukupno, metod_raspodele, datum
		FROM nabavke WHERE id = ?`, id).Scan(
		&n.ID, &dobavljacID, &napomena, &n.Ukupno, &metod, &n.Datum,
	)
	if err != nil {
		return nil, fmt.Errorf("ntech: NabavkaRepo.DohvatiID: %w", err)
	}

	if dobavljacID.Valid {
		n.DobavljacID = &dobavljacID.Int64
	}
	n.Napomena = napomena.String
	n.MetodRaspodele = metod.String

	return &n, nil
}

// DohvatiStavke vraća sve stavke jedne nabavke sa nazivima artikala
func (r *NabavkaRepo) DohvatiStavke(ctx context.Context, nabavkaID int64) ([]model.StavkaSaArtiklom, error) {
	redovi, err := r.db.QueryContext(ctx, `
		SELECT
			s.id, s.nabavka_id, s.artikal_id, s.kolicina,
			s.cena_po_komadu, s.ukupno,
			a.naziv AS artikal_naziv
		FROM stavke_nabavke s
		JOIN artikli a ON s.artikal_id = a.id
		WHERE s.nabavka_id = ?
		ORDER BY s.id ASC`, nabavkaID)
	if err != nil {
		return nil, fmt.Errorf("ntech: NabavkaRepo.DohvatiStavke: %w", err)
	}
	defer redovi.Close()

	var rezultat []model.StavkaSaArtiklom
	for redovi.Next() {
		var s model.StavkaSaArtiklom
		err := redovi.Scan(
			&s.ID, &s.NabavkaID, &s.ArtikalID, &s.Kolicina,
			&s.CenaPoKomadu, &s.Ukupno,
			&s.ArtikalNaziv,
		)
		if err != nil {
			return nil, fmt.Errorf("ntech: NabavkaRepo.DohvatiStavke: scan: %w", err)
		}
		rezultat = append(rezultat, s)
	}

	return rezultat, nil
}

// DohvatiTroskove vraća sve zavisne troškove jedne nabavke
func (r *NabavkaRepo) DohvatiTroskove(ctx context.Context, nabavkaID int64) ([]model.NabavkaTrosak, error) {
	redovi, err := r.db.QueryContext(ctx, `
		SELECT id, nabavka_id, naziv, iznos
		FROM nabavka_troskovi
		WHERE nabavka_id = ?
		ORDER BY id ASC`, nabavkaID)
	if err != nil {
		return nil, fmt.Errorf("ntech: NabavkaRepo.DohvatiTroskove: %w", err)
	}
	defer redovi.Close()

	var rezultat []model.NabavkaTrosak
	for redovi.Next() {
		var t model.NabavkaTrosak
		if err := redovi.Scan(&t.ID, &t.NabavkaID, &t.Naziv, &t.Iznos); err != nil {
			return nil, fmt.Errorf("ntech: NabavkaRepo.DohvatiTroskove: scan: %w", err)
		}
		rezultat = append(rezultat, t)
	}

	return rezultat, nil
}

// Kreiraj upisuje novu nabavku sa svim stavkama i zavisnim troškovima u jednoj
// transakciji i ažurira stanje magacina
func (r *NabavkaRepo) Kreiraj(ctx context.Context, n *model.Nabavka, stavke []model.StavkaNabavke, troskovi []model.NabavkaTrosak) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("ntech: NabavkaRepo.Kreiraj: begin: %w", err)
	}
	defer tx.Rollback()

	// računamo ukupan iznos nabavke kao zbir svih stavki
	var ukupno float64
	for i := range stavke {
		stavke[i].Ukupno = float64(stavke[i].Kolicina) * stavke[i].CenaPoKomadu
		ukupno += stavke[i].Ukupno
	}

	// upisujemo zaglavlje nabavke
	rezultat, err := tx.ExecContext(ctx, `
		INSERT INTO nabavke (dobavljac_id, napomena, ukupno, metod_raspodele)
		VALUES (?, ?, ?, ?)`,
		nullInt64(n.DobavljacID), nullString(n.Napomena), ukupno, nullString(n.MetodRaspodele),
	)
	if err != nil {
		return 0, fmt.Errorf("ntech: NabavkaRepo.Kreiraj: insert nabavka: %w", err)
	}

	nabavkaID, err := rezultat.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("ntech: NabavkaRepo.Kreiraj: last insert id: %w", err)
	}

	// upisujemo zavisne troškove (ako ih ima)
	for _, t := range troskovi {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO nabavka_troskovi (nabavka_id, naziv, iznos)
			VALUES (?, ?, ?)`,
			nabavkaID, t.Naziv, t.Iznos,
		)
		if err != nil {
			return 0, fmt.Errorf("ntech: NabavkaRepo.Kreiraj: insert trošak: %w", err)
		}
	}

	// upisujemo svaku stavku i ažuriramo stanje artikla u magacinu
	for _, s := range stavke {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO stavke_nabavke (nabavka_id, artikal_id, kolicina, cena_po_komadu, ukupno)
			VALUES (?, ?, ?, ?, ?)`,
			nabavkaID, s.ArtikalID, s.Kolicina, s.CenaPoKomadu, s.Ukupno,
		)
		if err != nil {
			return 0, fmt.Errorf("ntech: NabavkaRepo.Kreiraj: insert stavka: %w", err)
		}

		_, err = tx.ExecContext(ctx,
			"UPDATE artikli SET kolicina = kolicina + ? WHERE id = ?",
			s.Kolicina, s.ArtikalID,
		)
		if err != nil {
			return 0, fmt.Errorf("ntech: NabavkaRepo.Kreiraj: update kolicina: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("ntech: NabavkaRepo.Kreiraj: commit: %w", err)
	}

	return nabavkaID, nil
}

// Obrisi briše nabavku po ID-u — stavke se brišu automatski (ON DELETE CASCADE)
// Napomena: brisanje ne vraća količine artikala u magacin
func (r *NabavkaRepo) Obrisi(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM nabavke WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("ntech: NabavkaRepo.Obrisi: %w", err)
	}
	return nil
}
