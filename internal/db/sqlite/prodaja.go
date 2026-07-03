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

// SledeciBroj generiše sledeći broj naloga u formatu PR-GGMM-NNNN
// (GG dvocifrena godina, MM mesec); brojač NNNN se resetuje svakog meseca
func (r *ProdajaRepo) SledeciBroj(ctx context.Context) (string, error) {
	sada := time.Now()
	// prefiks "PR-GGMM-" je dug 8 karaktera, pa brojač počinje od 9. karaktera
	prefiks := fmt.Sprintf("PR-%02d%02d-", sada.Year()%100, int(sada.Month()))
	uzorak := prefiks + "%"

	var sledeci int
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(CAST(SUBSTR(broj_naloga, 9) AS INTEGER)), 0) + 1
		FROM prodajni_nalozi
		WHERE broj_naloga LIKE ?`, uzorak).Scan(&sledeci)
	if err != nil {
		return "", fmt.Errorf("ntech: ProdajaRepo.SledeciBroj: %w", err)
	}

	return fmt.Sprintf("%s%04d", prefiks, sledeci), nil
}

// Lista vraća listu prodajnih naloga sa imenom klijenta, po zadatom filteru
func (r *ProdajaRepo) Lista(ctx context.Context, filter db.ProdajaFilter) ([]model.ProdajniNalogSaDetaljem, error) {
	upit := `
		SELECT
			pn.id, pn.klijent_id, pn.broj_naloga, pn.napomena, pn.ukupno,
			pn.nacin_placanja, pn.stornirano, pn.datum,
			COALESCE(kp.naziv, '') AS klijent_naziv
		FROM prodajni_nalozi pn
		LEFT JOIN klijent_prikaz kp ON kp.id = pn.klijent_id
		WHERE 1=1`

	args := []any{}

	if filter.Pretraga != "" {
		upit += " AND (pn.broj_naloga LIKE ? OR kp.naziv LIKE ? OR pn.napomena LIKE ?)"
		args = append(args, "%"+filter.Pretraga+"%", "%"+filter.Pretraga+"%", "%"+filter.Pretraga+"%")
	}
	// pn.datum se čuva preko time.Time.String() (uključuje monotonic sufiks i naziv zone),
	// pa ga SQLite-ova date() funkcija ne parsira ispravno — poredimo samo ISO datum na početku stringa
	if filter.Od != "" {
		upit += " AND substr(pn.datum, 1, 10) >= ?"
		args = append(args, filter.Od)
	}
	if filter.Do != "" {
		upit += " AND substr(pn.datum, 1, 10) <= ?"
		args = append(args, filter.Do)
	}
	if filter.SamoStornirano {
		upit += " AND pn.stornirano = 1"
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
		SELECT sp.id, sp.nalog_id, sp.artikal_id, sp.kolicina, sp.cena_po_komadu,
		       sp.popust_procenat, sp.ukupno,
		       sp.pdv_stopa, sp.pdv_iznos, sp.cena_bez_pdv,
		       a.naziv, a.jedinica_mere, COALESCE(k.naziv, '') AS kategorija_naziv
		FROM stavke_prodaje sp
		JOIN artikli a ON a.id = sp.artikal_id
		LEFT JOIN kategorije k ON k.id = a.kategorija_id
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
			&s.CenaPoKomadu, &s.PopustProcenat, &s.Ukupno,
			&s.PdvStopa, &s.PdvIznos, &s.CenaBezPdv,
			&s.ArtikalNaziv, &s.JedinicaMere, &s.KategorijaNaziv,
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
	for i := range stavke {
		s := stavke[i]
		var naziv, tip string
		var stanjePre int
		var nabavnaCena float64
		err := tx.QueryRowContext(ctx,
			"SELECT naziv, kolicina, tip, nabavna_cena FROM artikli WHERE id = ?", s.ArtikalID,
		).Scan(&naziv, &stanjePre, &tip, &nabavnaCena)
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

		// popust umanjuje cenu po komadu pre računanja ukupnog i PDV-a
		cenaPoslePopusta := s.CenaPoKomadu
		if s.PopustProcenat > 0 {
			cenaPoslePopusta = cenaPoslePopusta * (1 - s.PopustProcenat/100)
		}
		ukupnoStavke := float64(s.Kolicina) * cenaPoslePopusta

		// CenaPoKomadu je cena za naplatu: kod punog PDV obveznika bruto (sa PDV-om,
		// PDV se izdvaja naniže), kod firme na evidenciji nema PDV-a (PdvStopa=0, pa je bez razlike).
		cenaBezPdv := s.CenaBezPdv
		pdvIznos := s.PdvIznos
		if cenaBezPdv == 0 {
			cenaBezPdv = cenaPoslePopusta / (1 + s.PdvStopa/100)
			pdvIznos = cenaPoslePopusta - cenaBezPdv
		}
		// vraćamo izračunate vrednosti u slajs pozivaoca (npr. za auto-upis u KIR posle Kreiraj)
		stavke[i].CenaBezPdv = cenaBezPdv
		stavke[i].PdvIznos = pdvIznos
		stavke[i].Ukupno = ukupnoStavke
		_, err = tx.ExecContext(ctx, `
			INSERT INTO stavke_prodaje
				(nalog_id, artikal_id, kolicina, cena_po_komadu, popust_procenat, ukupno, pdv_stopa, pdv_iznos, cena_bez_pdv, nabavna_cena)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			nalogID, s.ArtikalID, s.Kolicina, s.CenaPoKomadu, s.PopustProcenat, ukupnoStavke,
			s.PdvStopa, pdvIznos, cenaBezPdv, nabavnaCena,
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
func (r *ProdajaRepo) Obrisi(ctx context.Context, id int64, korisnikID *int64) error {
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
			var stanjePre int
			var tip string
			err := tx.QueryRowContext(ctx,
				"SELECT kolicina, tip FROM artikli WHERE id = ?", p.artikalID,
			).Scan(&stanjePre, &tip)
			if err != nil {
				return fmt.Errorf("ntech: ProdajaRepo.Obrisi: dohvati stanje: %w", err)
			}
			if !(tip == model.TipProizvod || tip == "") {
				continue
			}

			stanjePosle := stanjePre + p.kolicina
			_, err = tx.ExecContext(ctx,
				"UPDATE artikli SET kolicina = ? WHERE id = ?", stanjePosle, p.artikalID,
			)
			if err != nil {
				return fmt.Errorf("ntech: ProdajaRepo.Obrisi: vrati stanje: %w", err)
			}

			err = zabeleziMagacinPromenu(ctx, tx, p.artikalID, model.PromenaPovracaj,
				p.kolicina, stanjePre, stanjePosle, id, korisnikID, "brisanje prodajnog naloga")
			if err != nil {
				return fmt.Errorf("ntech: ProdajaRepo.Obrisi: magacin: %w", err)
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

// DnevniPrometMaloprodaje vraća zbrojene iznose maloprodajnih stavki za zadati dan.
// datum je string oblika "YYYY-MM-DD". Isključuje stornirane naloge i B2B (sa klijentom).
func (r *ProdajaRepo) DnevniPrometMaloprodaje(ctx context.Context, datum string) (model.DnevniPrometKir, error) {
	var p model.DnevniPrometKir

	// broj naloga tog dana
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM prodajni_nalozi
		WHERE klijent_id IS NULL AND stornirano = 0 AND DATE(datum) = ?`, datum,
	).Scan(&p.BrojNaloga)
	if err != nil {
		return p, fmt.Errorf("ntech: ProdajaRepo.DnevniPrometMaloprodaje: count: %w", err)
	}

	// stavke grupisane po PDV stopi — osnovica i PDV se čitaju iz već izračunatih
	// cena_bez_pdv/pdv_iznos kolona (ukupno je bruto iznos za naplatu, ne osnovica)
	redovi, err := r.db.QueryContext(ctx, `
		SELECT s.pdv_stopa, SUM(s.cena_bez_pdv * s.kolicina), SUM(s.pdv_iznos * s.kolicina)
		FROM stavke_prodaje s
		JOIN prodajni_nalozi p ON p.id = s.nalog_id
		WHERE p.klijent_id IS NULL AND p.stornirano = 0 AND DATE(p.datum) = ?
		GROUP BY s.pdv_stopa`, datum,
	)
	if err != nil {
		return p, fmt.Errorf("ntech: ProdajaRepo.DnevniPrometMaloprodaje: query: %w", err)
	}
	defer redovi.Close()

	for redovi.Next() {
		var stopa, osnovica, pdv float64
		if err := redovi.Scan(&stopa, &osnovica, &pdv); err != nil {
			return p, fmt.Errorf("ntech: ProdajaRepo.DnevniPrometMaloprodaje: scan: %w", err)
		}
		switch {
		case stopa >= 19.9 && stopa <= 20.1:
			p.OsnovicaOpsta += osnovica
			p.PdvOpsta += pdv
		case stopa >= 9.9 && stopa <= 10.1:
			p.OsnovicaPosebna += osnovica
			p.PdvPosebna += pdv
		}
	}
	return p, redovi.Err()
}
