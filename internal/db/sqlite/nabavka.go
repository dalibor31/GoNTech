package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

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
			n.id, n.dobavljac_id, n.napomena, n.ukupno, n.metod_raspodele,
			n.stornirano, n.razlog_storniranja, n.datum,
			n.broj_racuna, n.datum_racuna, n.pdv_iznos, n.datum_placanja,
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
		var napomena, metod, razlogStorniranja, brojRacuna, datumRacuna, datumPlacanja sql.NullString

		err := redovi.Scan(
			&n.ID, &dobavljacID, &napomena, &n.Ukupno, &metod,
			&n.Stornirano, &razlogStorniranja, &n.Datum,
			&brojRacuna, &datumRacuna, &n.PdvIznos, &datumPlacanja,
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
		n.RazlogStorniranja = razlogStorniranja.String
		n.BrojRacuna = brojRacuna.String
		if datumRacuna.Valid && datumRacuna.String != "" {
			if t, e := time.Parse("2006-01-02", datumRacuna.String[:10]); e == nil {
				n.DatumRacuna = &t
			}
		}
		if datumPlacanja.Valid && datumPlacanja.String != "" {
			if t, e := time.Parse("2006-01-02", datumPlacanja.String[:10]); e == nil {
				n.DatumPlacanja = &t
			}
		}

		rezultat = append(rezultat, n)
	}

	return rezultat, nil
}

// DohvatiID vraća zaglavlje jedne nabavke po ID-u
func (r *NabavkaRepo) DohvatiID(ctx context.Context, id int64) (*model.Nabavka, error) {
	var n model.Nabavka
	var dobavljacID sql.NullInt64
	var napomena, metod, razlogStorniranja, brojRacuna, datumRacuna, datumPlacanja sql.NullString

	err := r.db.QueryRowContext(ctx, `
		SELECT id, dobavljac_id, napomena, ukupno, metod_raspodele,
		       stornirano, razlog_storniranja, datum,
		       broj_racuna, datum_racuna, pdv_iznos, datum_placanja
		FROM nabavke WHERE id = ?`, id).Scan(
		&n.ID, &dobavljacID, &napomena, &n.Ukupno, &metod,
		&n.Stornirano, &razlogStorniranja, &n.Datum,
		&brojRacuna, &datumRacuna, &n.PdvIznos, &datumPlacanja,
	)
	if err != nil {
		return nil, fmt.Errorf("ntech: NabavkaRepo.DohvatiID: %w", err)
	}

	if dobavljacID.Valid {
		n.DobavljacID = &dobavljacID.Int64
	}
	n.Napomena = napomena.String
	n.MetodRaspodele = metod.String
	n.RazlogStorniranja = razlogStorniranja.String
	n.BrojRacuna = brojRacuna.String
	if datumRacuna.Valid && datumRacuna.String != "" {
		if t, e := time.Parse("2006-01-02", datumRacuna.String[:10]); e == nil {
			n.DatumRacuna = &t
		}
	}
	if datumPlacanja.Valid && datumPlacanja.String != "" {
		if t, e := time.Parse("2006-01-02", datumPlacanja.String[:10]); e == nil {
			n.DatumPlacanja = &t
		}
	}

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
func (r *NabavkaRepo) Kreiraj(ctx context.Context, n *model.Nabavka, stavke []model.StavkaNabavke, troskovi []model.NabavkaTrosak, korisnikID *int64) (int64, error) {
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
		INSERT INTO nabavke (dobavljac_id, napomena, ukupno, metod_raspodele, broj_racuna, datum_racuna, pdv_iznos, datum_placanja)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		nullInt64(n.DobavljacID), nullString(n.Napomena), ukupno, nullString(n.MetodRaspodele),
		nullString(n.BrojRacuna), nullDateString(n.DatumRacuna), n.PdvIznos, nullDateString(n.DatumPlacanja),
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

		var stanjePre int
		var staraNabavnaCena float64
		err = tx.QueryRowContext(ctx,
			"SELECT kolicina, nabavna_cena FROM artikli WHERE id = ?", s.ArtikalID,
		).Scan(&stanjePre, &staraNabavnaCena)
		if err != nil {
			return 0, fmt.Errorf("ntech: NabavkaRepo.Kreiraj: dohvati stanje: %w", err)
		}

		// ponderisana prosečna nabavna cena (MRS 2)
		stanjePosle := stanjePre + s.Kolicina
		var novaProsecna float64
		if stanjePosle > 0 {
			novaProsecna = (float64(stanjePre)*staraNabavnaCena + float64(s.Kolicina)*s.CenaPoKomadu) / float64(stanjePosle)
		} else {
			novaProsecna = s.CenaPoKomadu
		}

		_, err = tx.ExecContext(ctx,
			"UPDATE artikli SET kolicina = ?, nabavna_cena = ? WHERE id = ?",
			stanjePosle, novaProsecna, s.ArtikalID,
		)
		if err != nil {
			return 0, fmt.Errorf("ntech: NabavkaRepo.Kreiraj: update kolicina: %w", err)
		}

		if err = zabeleziMagacinPromenu(ctx, tx, s.ArtikalID, model.PromenaUlazNabavka,
			s.Kolicina, stanjePre, stanjePosle, nabavkaID, korisnikID, ""); err != nil {
			return 0, fmt.Errorf("ntech: NabavkaRepo.Kreiraj: magacin: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("ntech: NabavkaRepo.Kreiraj: commit: %w", err)
	}

	return nabavkaID, nil
}

// Storno označava nabavku kao storniranu, vraća količine artikala na stanje i beleži
// korekciju u magacinski trag. Nabavna cena se NE računa unazad — ostaje kakva jeste.
func (r *NabavkaRepo) Storno(ctx context.Context, id int64, razlog string, korisnikID *int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ntech: NabavkaRepo.Storno: begin: %w", err)
	}
	defer tx.Rollback()

	// provera da li je već stornirano
	var stornirano bool
	if err := tx.QueryRowContext(ctx,
		"SELECT stornirano FROM nabavke WHERE id = ?", id,
	).Scan(&stornirano); err != nil {
		return fmt.Errorf("ntech: NabavkaRepo.Storno: provera: %w", err)
	}
	if stornirano {
		return fmt.Errorf("ntech: NabavkaRepo.Storno: nabavka je već stornirana")
	}

	// učitaj stavke
	redovi, err := tx.QueryContext(ctx,
		"SELECT artikal_id, kolicina FROM stavke_nabavke WHERE nabavka_id = ?", id)
	if err != nil {
		return fmt.Errorf("ntech: NabavkaRepo.Storno: dohvati stavke: %w", err)
	}
	type stavka struct {
		artikalID int64
		kolicina  int
	}
	var stavke []stavka
	for redovi.Next() {
		var s stavka
		if err := redovi.Scan(&s.artikalID, &s.kolicina); err != nil {
			redovi.Close()
			return fmt.Errorf("ntech: NabavkaRepo.Storno: scan stavka: %w", err)
		}
		stavke = append(stavke, s)
	}
	redovi.Close()

	// vrati količine na stanje i zabeleži korekciju
	for _, s := range stavke {
		var stanjePre int
		if err := tx.QueryRowContext(ctx,
			"SELECT kolicina FROM artikli WHERE id = ?", s.artikalID,
		).Scan(&stanjePre); err != nil {
			return fmt.Errorf("ntech: NabavkaRepo.Storno: dohvati stanje: %w", err)
		}
		stanjePosle := stanjePre - s.kolicina
		if stanjePosle < 0 {
			stanjePosle = 0
		}
		if _, err := tx.ExecContext(ctx,
			"UPDATE artikli SET kolicina = ? WHERE id = ?", stanjePosle, s.artikalID,
		); err != nil {
			return fmt.Errorf("ntech: NabavkaRepo.Storno: update stanje: %w", err)
		}
		if err := zabeleziMagacinPromenu(ctx, tx, s.artikalID, model.PromenaKorekcija,
			-s.kolicina, stanjePre, stanjePosle, id, korisnikID, "storno nabavke: "+razlog); err != nil {
			return fmt.Errorf("ntech: NabavkaRepo.Storno: magacin: %w", err)
		}
	}

	if _, err := tx.ExecContext(ctx,
		"UPDATE nabavke SET stornirano = 1, razlog_storniranja = ? WHERE id = ?",
		nullString(razlog), id,
	); err != nil {
		return fmt.Errorf("ntech: NabavkaRepo.Storno: update nabavka: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("ntech: NabavkaRepo.Storno: commit: %w", err)
	}
	return nil
}
