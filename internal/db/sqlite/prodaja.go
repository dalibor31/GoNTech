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
			pn.id, pn.klijent_id, pn.broj_naloga, pn.napomena, pn.ukupno,
			pn.nacin_placanja, pn.stornirano, pn.datum,
			COALESCE(kp.naziv, '') AS klijent_naziv
		FROM prodajni_nalozi pn
		LEFT JOIN klijent_prikaz kp ON kp.id = pn.klijent_id
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
			&n.ID, &klijentID, &n.BrojNaloga, &napomena, &n.Ukupno,
			&n.NacinPlacanja, &n.Stornirano, &n.Datum,
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
		SELECT id, klijent_id, broj_naloga, napomena, ukupno,
		       nacin_placanja, stornirano, razlog_storniranja, datum
		FROM prodajni_nalozi WHERE id = ?`, id)

	var n model.ProdajniNalog
	var klijentID sql.NullInt64
	var napomena, razlogStorniranja sql.NullString

	err := red.Scan(
		&n.ID, &klijentID, &n.BrojNaloga, &napomena, &n.Ukupno,
		&n.NacinPlacanja, &n.Stornirano, &razlogStorniranja, &n.Datum,
	)
	if err != nil {
		return nil, fmt.Errorf("ntech: ProdajaRepo.DohvatiID: %w", err)
	}

	if klijentID.Valid {
		v := klijentID.Int64
		n.KlijentID = &v
	}
	n.Napomena = napomena.String
	n.RazlogStorniranja = razlogStorniranja.String

	return &n, nil
}

// DohvatiStavke vraća stavke prodaje sa nazivima artikala i PDV podacima za dati nalog
func (r *ProdajaRepo) DohvatiStavke(ctx context.Context, nalogID int64) ([]model.StavkaProdajeSaArtiklom, error) {
	redovi, err := r.db.QueryContext(ctx, `
		SELECT sp.id, sp.nalog_id, sp.artikal_id, sp.kolicina, sp.cena_po_komadu, sp.ukupno,
		       sp.pdv_stopa, sp.pdv_iznos, sp.cena_bez_pdv, a.naziv, a.jedinica_mere
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
			&s.CenaPoKomadu, &s.Ukupno,
			&s.PdvStopa, &s.PdvIznos, &s.CenaBezPdv,
			&s.ArtikalNaziv, &s.JedinicaMere,
		)
		if err != nil {
			return nil, fmt.Errorf("ntech: ProdajaRepo.DohvatiStavke: scan: %w", err)
		}
		stavke = append(stavke, s)
	}

	return stavke, nil
}

// Kreiraj upisuje novi prodajni nalog u bazu u okviru jedne transakcije.
// Za svaku stavku proverava stanje u magacinu, smanjuje ga i beleži promenu.
// Ako bilo koji artikal nema dovoljno stanja, vraća ErrNedovoljnoKolicine.
func (r *ProdajaRepo) Kreiraj(ctx context.Context, n *model.ProdajniNalog, stavke []model.StavkaProdaje, korisnikID *int64) (int64, error) {
	if n.NacinPlacanja == "" {
		n.NacinPlacanja = "gotovina"
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("ntech: ProdajaRepo.Kreiraj: begin tx: %w", err)
	}
	defer tx.Rollback()

	// insert zaglavlja naloga pre stavki da bismo imali nalogID za magacin
	rezultat, err := tx.ExecContext(ctx, `
		INSERT INTO prodajni_nalozi (klijent_id, broj_naloga, napomena, ukupno, nacin_placanja, datum)
		VALUES (?, ?, ?, ?, ?, ?)`,
		nullInt64(n.KlijentID), n.BrojNaloga, nullString(n.Napomena), n.Ukupno, n.NacinPlacanja, n.Datum,
	)
	if err != nil {
		return 0, fmt.Errorf("ntech: ProdajaRepo.Kreiraj: insert nalog: %w", err)
	}

	nalogID, err := rezultat.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("ntech: ProdajaRepo.Kreiraj: last insert id: %w", err)
	}

	// provera stanja, smanjenje i insert stavki
	for _, s := range stavke {
		var naziv, tip string
		var stanjePre int
		err := tx.QueryRowContext(ctx,
			"SELECT naziv, kolicina, tip FROM artikli WHERE id = ?", s.ArtikalID,
		).Scan(&naziv, &stanjePre, &tip)
		if err != nil {
			return 0, fmt.Errorf("ntech: ProdajaRepo.Kreiraj: dohvati artikal: %w", err)
		}

		// usluge i troškovi ne prate lager — ne proveravaju se i ne umanjuju
		pratiLager := tip == model.TipProizvod || tip == ""
		stanjePosle := stanjePre
		if pratiLager {
			if stanjePre < s.Kolicina {
				return 0, &db.ErrNedovoljnoKolicine{ArtikalNaziv: naziv}
			}
			stanjePosle = stanjePre - s.Kolicina
			_, err = tx.ExecContext(ctx,
				"UPDATE artikli SET kolicina = ? WHERE id = ?", stanjePosle, s.ArtikalID,
			)
			if err != nil {
				return 0, fmt.Errorf("ntech: ProdajaRepo.Kreiraj: update stanje: %w", err)
			}
		}

		// PDV računamo iz cene ako nije eksplicitno postavljeno
		cenaBezPdv := s.CenaBezPdv
		pdvIznos := s.PdvIznos
		if cenaBezPdv == 0 && s.PdvStopa > 0 {
			cenaBezPdv = s.CenaPoKomadu / (1 + s.PdvStopa/100)
			pdvIznos = s.CenaPoKomadu - cenaBezPdv
		}

		ukupnoStavke := float64(s.Kolicina) * s.CenaPoKomadu
		_, err = tx.ExecContext(ctx, `
			INSERT INTO stavke_prodaje
				(nalog_id, artikal_id, kolicina, cena_po_komadu, ukupno, pdv_stopa, pdv_iznos, cena_bez_pdv)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			nalogID, s.ArtikalID, s.Kolicina, s.CenaPoKomadu, ukupnoStavke,
			s.PdvStopa, pdvIznos, cenaBezPdv,
		)
		if err != nil {
			return 0, fmt.Errorf("ntech: ProdajaRepo.Kreiraj: insert stavka: %w", err)
		}

		// magacinsku promenu beležimo samo za artikle koji prate lager
		if pratiLager {
			err = zabeleziMagacinPromenu(ctx, tx, s.ArtikalID, model.PromenaIzlazProdaja,
				-s.Kolicina, stanjePre, stanjePosle, nalogID, korisnikID, "")
			if err != nil {
				return 0, fmt.Errorf("ntech: ProdajaRepo.Kreiraj: magacin: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("ntech: ProdajaRepo.Kreiraj: commit: %w", err)
	}

	return nalogID, nil
}

// Storno označava prodajni nalog kao storniran i vraća sve artikle na stanje
func (r *ProdajaRepo) Storno(ctx context.Context, id int64, razlog string, korisnikID *int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ntech: ProdajaRepo.Storno: begin tx: %w", err)
	}
	defer tx.Rollback()

	// proverava da li je već storniran
	var stornirano bool
	err = tx.QueryRowContext(ctx, "SELECT stornirano FROM prodajni_nalozi WHERE id = ?", id).Scan(&stornirano)
	if err != nil {
		return fmt.Errorf("ntech: ProdajaRepo.Storno: provera: %w", err)
	}
	if stornirano {
		return fmt.Errorf("ntech: ProdajaRepo.Storno: nalog je već storniran")
	}

	// vraćanje stanja u magacin
	redovi, err := tx.QueryContext(ctx,
		"SELECT artikal_id, kolicina FROM stavke_prodaje WHERE nalog_id = ?", id)
	if err != nil {
		return fmt.Errorf("ntech: ProdajaRepo.Storno: dohvati stavke: %w", err)
	}

	type stavkaPovrat struct {
		artikalID int64
		kolicina  int
	}
	var stavke []stavkaPovrat
	for redovi.Next() {
		var p stavkaPovrat
		if err := redovi.Scan(&p.artikalID, &p.kolicina); err != nil {
			redovi.Close()
			return fmt.Errorf("ntech: ProdajaRepo.Storno: scan stavke: %w", err)
		}
		stavke = append(stavke, p)
	}
	redovi.Close()

	for _, p := range stavke {
		var stanjePre int
		var tip string
		err := tx.QueryRowContext(ctx, "SELECT kolicina, tip FROM artikli WHERE id = ?", p.artikalID).Scan(&stanjePre, &tip)
		if err != nil {
			return fmt.Errorf("ntech: ProdajaRepo.Storno: dohvati stanje: %w", err)
		}

		// usluge i troškovi nemaju stanje na lageru — preskačemo povraćaj
		if !(tip == model.TipProizvod || tip == "") {
			continue
		}

		stanjePosle := stanjePre + p.kolicina
		_, err = tx.ExecContext(ctx,
			"UPDATE artikli SET kolicina = ? WHERE id = ?", stanjePosle, p.artikalID,
		)
		if err != nil {
			return fmt.Errorf("ntech: ProdajaRepo.Storno: vrati stanje: %w", err)
		}

		err = zabeleziMagacinPromenu(ctx, tx, p.artikalID, model.PromenaPovracaj,
			p.kolicina, stanjePre, stanjePosle, id, korisnikID, razlog)
		if err != nil {
			return fmt.Errorf("ntech: ProdajaRepo.Storno: magacin: %w", err)
		}
	}

	_, err = tx.ExecContext(ctx,
		"UPDATE prodajni_nalozi SET stornirano = 1, razlog_storniranja = ? WHERE id = ?",
		nullString(razlog), id,
	)
	if err != nil {
		return fmt.Errorf("ntech: ProdajaRepo.Storno: update nalog: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("ntech: ProdajaRepo.Storno: commit: %w", err)
	}

	return nil
}

// Obrisi briše prodajni nalog i vraća količine artikala na stanje (u transakciji)
func (r *ProdajaRepo) Obrisi(ctx context.Context, id int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ntech: ProdajaRepo.Obrisi: begin tx: %w", err)
	}
	defer tx.Rollback()

	// vraćanje stanja u magacin samo ako nalog nije storniran (storno je već vratio)
	var stornirano bool
	err = tx.QueryRowContext(ctx, "SELECT stornirano FROM prodajni_nalozi WHERE id = ?", id).Scan(&stornirano)
	if err != nil {
		return fmt.Errorf("ntech: ProdajaRepo.Obrisi: provera: %w", err)
	}

	if !stornirano {
		redovi, err := tx.QueryContext(ctx,
			"SELECT artikal_id, kolicina FROM stavke_prodaje WHERE nalog_id = ?", id)
		if err != nil {
			return fmt.Errorf("ntech: ProdajaRepo.Obrisi: dohvati stavke: %w", err)
		}

		type povrat struct {
			artikalID int64
			kolicina  int
		}
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
			// usluge i troškovi nemaju stanje — vraćamo samo proizvodima
			_, err := tx.ExecContext(ctx,
				"UPDATE artikli SET kolicina = kolicina + ? WHERE id = ? AND (tip = 'proizvod' OR tip = '')",
				p.kolicina, p.artikalID,
			)
			if err != nil {
				return fmt.Errorf("ntech: ProdajaRepo.Obrisi: vrati stanje: %w", err)
			}
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
