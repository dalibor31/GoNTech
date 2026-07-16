package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"ntech/internal/db"
	"ntech/internal/model"
)

// NivelacijaRepo je SQLite implementacija NivelacijaRepository interfejsa
type NivelacijaRepo struct {
	db *sql.DB
}

// NoviNivelacijaRepo kreira novi NivelacijaRepo
func NoviNivelacijaRepo(db *sql.DB) *NivelacijaRepo {
	return &NivelacijaRepo{db: db}
}

// PromeniCenu transakciono menja prodajnu cenu artikla i upisuje nivelacioni zapis.
// Stara cena se čita iz baze unutar transakcije; izvor je "rucno".
func (r *NivelacijaRepo) PromeniCenu(ctx context.Context, artikalID int64, novaCena float64, razlog string, korisnikID *int64) (*model.Nivelacija, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("ntech: NivelacijaRepo.PromeniCenu: begin: %w", err)
	}
	defer tx.Rollback()

	var stara float64
	err = tx.QueryRowContext(ctx, "SELECT prodajna_cena FROM artikli WHERE id = ?", artikalID).Scan(&stara)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, db.ErrArtikalNePostoji
	}
	if err != nil {
		return nil, fmt.Errorf("ntech: NivelacijaRepo.PromeniCenu: čitanje cene: %w", err)
	}

	if _, err = tx.ExecContext(ctx, "UPDATE artikli SET prodajna_cena = ? WHERE id = ?", novaCena, artikalID); err != nil {
		return nil, fmt.Errorf("ntech: NivelacijaRepo.PromeniCenu: update cene: %w", err)
	}

	sada := time.Now()
	rez, err := tx.ExecContext(ctx, `
		INSERT INTO nivelacije (artikal_id, stara_cena, nova_cena, razlog, izvor, korisnik_id, datum)
		VALUES (?, ?, ?, ?, 'rucno', ?, ?)`,
		artikalID, stara, novaCena, razlog, izvorIDArg(korisnikID), sada)
	if err != nil {
		return nil, fmt.Errorf("ntech: NivelacijaRepo.PromeniCenu: upis nivelacije: %w", err)
	}
	id, _ := rez.LastInsertId()

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("ntech: NivelacijaRepo.PromeniCenu: commit: %w", err)
	}

	return &model.Nivelacija{
		ID: id, ArtikalID: artikalID, StaraCena: stara, NovaCena: novaCena,
		Razlog: razlog, Izvor: "rucno", KorisnikID: korisnikID, Datum: sada,
	}, nil
}

// Kreiraj upisuje gotov nivelacioni zapis (npr. auto-trag pri izmeni artikla).
func (r *NivelacijaRepo) Kreiraj(ctx context.Context, n *model.Nivelacija) (int64, error) {
	izvor := n.Izvor
	if izvor == "" {
		izvor = "rucno"
	}
	datum := n.Datum
	if datum.IsZero() {
		datum = time.Now()
	}
	rez, err := r.db.ExecContext(ctx, `
		INSERT INTO nivelacije (artikal_id, stara_cena, nova_cena, razlog, izvor, korisnik_id, datum)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		n.ArtikalID, n.StaraCena, n.NovaCena, n.Razlog, izvor, izvorIDArg(n.KorisnikID), datum)
	if err != nil {
		return 0, fmt.Errorf("ntech: NivelacijaRepo.Kreiraj: %w", err)
	}
	id, err := rez.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("ntech: NivelacijaRepo.Kreiraj: id: %w", err)
	}
	return id, nil
}

// Lista vraća nivelacije u periodu (po datumu); nulti datum znači bez granice.
func (r *NivelacijaRepo) Lista(ctx context.Context, od, do time.Time) ([]model.Nivelacija, error) {
	upit := nivelacijaSelect + " WHERE 1=1"
	args := []any{}
	if !od.IsZero() {
		upit += " AND n.datum >= ?"
		args = append(args, od)
	}
	if !do.IsZero() {
		upit += " AND n.datum <= ?"
		args = append(args, do)
	}
	upit += " ORDER BY n.datum DESC, n.id DESC"
	return r.upitVisestruko(ctx, upit, args...)
}

// ListaZaArtikal vraća sve nivelacije jednog artikla (najnovije prvo).
func (r *NivelacijaRepo) ListaZaArtikal(ctx context.Context, artikalID int64) ([]model.Nivelacija, error) {
	upit := nivelacijaSelect + " WHERE n.artikal_id = ? ORDER BY n.datum DESC, n.id DESC"
	return r.upitVisestruko(ctx, upit, artikalID)
}

// nivelacijaSelect je zajednički SELECT sa JOIN-ovima na artikle i korisnike (za prikaz).
const nivelacijaSelect = `
	SELECT n.id, n.artikal_id, COALESCE(a.naziv, ''), n.stara_cena, n.nova_cena,
	       COALESCE(n.razlog, ''), n.izvor, n.korisnik_id, COALESCE(k.korisnicko_ime, ''),
	       n.datum, n.datum_unosa
	FROM nivelacije n
	LEFT JOIN artikli a ON a.id = n.artikal_id
	LEFT JOIN korisnici k ON k.id = n.korisnik_id`

// upitVisestruko izvršava SELECT i skenira sve redove u listu nivelacija.
func (r *NivelacijaRepo) upitVisestruko(ctx context.Context, upit string, args ...any) ([]model.Nivelacija, error) {
	redovi, err := r.db.QueryContext(ctx, upit, args...)
	if err != nil {
		return nil, fmt.Errorf("ntech: NivelacijaRepo: %w", err)
	}
	defer redovi.Close()

	var rezultat []model.Nivelacija
	for redovi.Next() {
		var n model.Nivelacija
		var korisnikID sql.NullInt64
		if err := redovi.Scan(
			&n.ID, &n.ArtikalID, &n.ArtikalNaziv, &n.StaraCena, &n.NovaCena,
			&n.Razlog, &n.Izvor, &korisnikID, &n.KorisnikIme, &n.Datum, &n.DatumUnosa,
		); err != nil {
			return nil, fmt.Errorf("ntech: NivelacijaRepo: scan: %w", err)
		}
		if korisnikID.Valid {
			n.KorisnikID = &korisnikID.Int64
		}
		rezultat = append(rezultat, n)
	}
	if err := redovi.Err(); err != nil {
		return nil, fmt.Errorf("ntech: NivelacijaRepo: %w", err)
	}
	return rezultat, nil
}
