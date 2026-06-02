package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"ntech/internal/db"
	"ntech/internal/model"
)

// ProdajaRepo je SQLite implementacija ProdajaRepository interfejsa
type ProdajaRepo struct {
	db *sql.DB
}

// NoviProdajaRepo kreira novi ProdajaRepo
func NoviProdajaRepo(db *sql.DB) *ProdajaRepo {
	return &ProdajaRepo{db: db}
}

// SledeciBroj generiše sledeći broj naloga u formatu PR-GGGG-NNNN
func (r *ProdajaRepo) SledeciBroj(ctx context.Context) (string, error) {
	godina := time.Now().Year()
	uzorak := fmt.Sprintf("PR-%d-%%", godina)

	var sledeci int
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(CAST(SUBSTR(broj_naloga, 9) AS INTEGER)), 0) + 1
		FROM prodajni_nalozi
		WHERE broj_naloga LIKE ?`, uzorak).Scan(&sledeci)
	if err != nil {
		return "", fmt.Errorf("ntech: ProdajaRepo.SledeciBroj: %w", err)
	}

	return fmt.Sprintf("PR-%d-%04d", godina, sledeci), nil
}

// Lista vraća listu prodajnih naloga sa imenom klijenta, opcionom pretragom
func (r *ProdajaRepo) Lista(ctx context.Context, pretraga string) ([]model.ProdajniNalogSaDetaljem, error) {
	upit := `
		SELECT
			pn.id, pn.klijent_id, pn.broj_naloga, pn.napomena, pn.ukupno, pn.datum,
			COALESCE(NULLIF(k.naziv_firme, ''), TRIM(COALESCE(k.ime, '') || ' ' || COALESCE(k.prezime, '')), '') AS klijent_naziv
		FROM prodajni_nalozi pn
		LEFT JOIN klijenti k ON k.id = pn.klijent_id
		WHERE 1=1`

	args := []any{}

	if pretraga != "" {
		upit += " AND pn.broj_naloga LIKE ?"
		args = append(args, "%"+pretraga+"%")
	}

	upit += " ORDER BY pn.datum DESC"

	redovi, err := r.db.QueryContext(ctx, upit, args...)
	if err != nil {
		return nil, fmt.Errorf("ntech: ProdajaRepo.Lista: %w", err)
	}
	defer redovi.Close()

	var rezultat []model.ProdajniNalogSaDetaljem
	for redovi.Next() {
		var n model.ProdajniNalogSaDetaljem
		var klijentID sql.NullInt64
		var napomena sql.NullString

		err := redovi.Scan(
			&n.ID, &klijentID, &n.BrojNaloga, &napomena, &n.Ukupno, &n.Datum,
			&n.KlijentNaziv,
		)
		if err != nil {
			return nil, fmt.Errorf("ntech: ProdajaRepo.Lista: scan: %w", err)
		}

		if klijentID.Valid {
			v := klijentID.Int64
			n.KlijentID = &v
		}
		n.Napomena = napomena.String

		rezultat = append(rezultat, n)
	}

	return rezultat, nil
}

// DohvatiID vraća jedan prodajni nalog po ID-u
func (r *ProdajaRepo) DohvatiID(ctx context.Context, id int64) (*model.ProdajniNalog, error) {
	red := r.db.QueryRowContext(ctx, `
		SELECT id, klijent_id, broj_naloga, napomena, ukupno, datum
		FROM prodajni_nalozi WHERE id = ?`, id)

	var n model.ProdajniNalog
	var klijentID sql.NullInt64
	var napomena sql.NullString

	err := red.Scan(&n.ID, &klijentID, &n.BrojNaloga, &napomena, &n.Ukupno, &n.Datum)
	if err != nil {
		return nil, fmt.Errorf("ntech: ProdajaRepo.DohvatiID: %w", err)
	}

	if klijentID.Valid {
		v := klijentID.Int64
		n.KlijentID = &v
	}
	n.Napomena = napomena.String

	return &n, nil
}

// DohvatiStavke vraća stavke prodaje sa nazivima artikala za dati nalog
func (r *ProdajaRepo) DohvatiStavke(ctx context.Context, nalogID int64) ([]model.StavkaProdajeSaArtiklom, error) {
	redovi, err := r.db.QueryContext(ctx, `
		SELECT sp.id, sp.nalog_id, sp.artikal_id, sp.kolicina, sp.cena_po_komadu, sp.ukupno,
		       a.naziv
		FROM stavke_prodaje sp
		JOIN artikli a ON a.id = sp.artikal_id
		WHERE sp.nalog_id = ?
		ORDER BY sp.id`, nalogID)
	if err != nil {
		return nil, fmt.Errorf("ntech: ProdajaRepo.DohvatiStavke: %w", err)
	}
	defer redovi.Close()

	var stavke []model.StavkaProdajeSaArtiklom
	for redovi.Next() {
		var s model.StavkaProdajeSaArtiklom
		err := redovi.Scan(
			&s.ID, &s.NalogID, &s.ArtikalID, &s.Kolicina,
			&s.CenaPoKomadu, &s.Ukupno, &s.ArtikalNaziv,
		)
		if err != nil {
			return nil, fmt.Errorf("ntech: ProdajaRepo.DohvatiStavke: scan: %w", err)
		}
		stavke = append(stavke, s)
	}

	return stavke, nil
}

// Kreiraj upisuje novi prodajni nalog u bazu u okviru jedne transakcije.
// Za svaku stavku proverava stanje u magacinu i smanjuje ga.
// Ako bilo koji artikal nema dovoljno stanja, vraća ErrNedovoljnoKolicine.
func (r *ProdajaRepo) Kreiraj(ctx context.Context, n *model.ProdajniNalog, stavke []model.StavkaProdaje) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("ntech: ProdajaRepo.Kreiraj: begin tx: %w", err)
	}
	defer tx.Rollback()

	// provera i smanjenje stanja za svaku stavku
	for _, s := range stavke {
		var naziv string
		var kolicinaNaStanju int
		err := tx.QueryRowContext(ctx,
			"SELECT naziv, kolicina FROM artikli WHERE id = ?", s.ArtikalID,
		).Scan(&naziv, &kolicinaNaStanju)
		if err != nil {
			return 0, fmt.Errorf("ntech: ProdajaRepo.Kreiraj: dohvati artikal: %w", err)
		}

		if kolicinaNaStanju < s.Kolicina {
			return 0, &db.ErrNedovoljnoKolicine{ArtikalNaziv: naziv}
		}

		_, err = tx.ExecContext(ctx,
			"UPDATE artikli SET kolicina = kolicina - ? WHERE id = ?",
			s.Kolicina, s.ArtikalID,
		)
		if err != nil {
			return 0, fmt.Errorf("ntech: ProdajaRepo.Kreiraj: update stanje: %w", err)
		}
	}

	// insert zaglavlja naloga
	rezultat, err := tx.ExecContext(ctx, `
		INSERT INTO prodajni_nalozi (klijent_id, broj_naloga, napomena, ukupno, datum)
		VALUES (?, ?, ?, ?, ?)`,
		nullInt64(n.KlijentID), n.BrojNaloga, nullString(n.Napomena), n.Ukupno, n.Datum,
	)
	if err != nil {
		return 0, fmt.Errorf("ntech: ProdajaRepo.Kreiraj: insert nalog: %w", err)
	}

	nalogID, err := rezultat.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("ntech: ProdajaRepo.Kreiraj: last insert id: %w", err)
	}

	// insert stavki
	for _, s := range stavke {
		ukupnoStavke := float64(s.Kolicina) * s.CenaPoKomadu
		_, err := tx.ExecContext(ctx, `
			INSERT INTO stavke_prodaje (nalog_id, artikal_id, kolicina, cena_po_komadu, ukupno)
			VALUES (?, ?, ?, ?, ?)`,
			nalogID, s.ArtikalID, s.Kolicina, s.CenaPoKomadu, ukupnoStavke,
		)
		if err != nil {
			return 0, fmt.Errorf("ntech: ProdajaRepo.Kreiraj: insert stavka: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("ntech: ProdajaRepo.Kreiraj: commit: %w", err)
	}

	return nalogID, nil
}

// Obrisi briše prodajni nalog i vraća količine artikala na stanje (u transakciji)
func (r *ProdajaRepo) Obrisi(ctx context.Context, id int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ntech: ProdajaRepo.Obrisi: begin tx: %w", err)
	}
	defer tx.Rollback()

	// vraćanje stanja u magacin
	redovi, err := tx.QueryContext(ctx,
		"SELECT artikal_id, kolicina FROM stavke_prodaje WHERE nalog_id = ?", id)
	if err != nil {
		return fmt.Errorf("ntech: ProdajaRepo.Obrisi: dohvati stavke: %w", err)
	}

	type povrat struct{ artikalID int64; kolicina int }
	var stavke []povrat
	for redovi.Next() {
		var p povrat
		if err := redovi.Scan(&p.artikalID, &p.kolicina); err != nil {
			redovi.Close()
			return fmt.Errorf("ntech: ProdajaRepo.Obrisi: scan stavke: %w", err)
		}
		stavke = append(stavke, p)
	}
	redovi.Close()

	for _, p := range stavke {
		_, err := tx.ExecContext(ctx,
			"UPDATE artikli SET kolicina = kolicina + ? WHERE id = ?",
			p.kolicina, p.artikalID,
		)
		if err != nil {
			return fmt.Errorf("ntech: ProdajaRepo.Obrisi: vrati stanje: %w", err)
		}
	}

	// CASCADE briše i stavke_prodaje
	_, err = tx.ExecContext(ctx, "DELETE FROM prodajni_nalozi WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("ntech: ProdajaRepo.Obrisi: delete: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("ntech: ProdajaRepo.Obrisi: commit: %w", err)
	}

	return nil
}
