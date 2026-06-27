package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"ntech/internal/model"
)

type sqliteIzvestajRepo struct{ db *sql.DB }

// NoviIzvestajRepo kreira SQLite implementaciju IzvestajRepository
func NoviIzvestajRepo(db *sql.DB) *sqliteIzvestajRepo {
	return &sqliteIzvestajRepo{db: db}
}

func (r *sqliteIzvestajRepo) BrojArtikala(ctx context.Context) (int, error) {
	var n int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM artikli`).Scan(&n); err != nil {
		return 0, fmt.Errorf("ntech: izvestaj.BrojArtikala: %w", err)
	}
	return n, nil
}

func (r *sqliteIzvestajRepo) BrojAktivnihServisa(ctx context.Context) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM servisni_nalozi WHERE status NOT IN ('Završeno', 'Preuzeto')`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("ntech: izvestaj.BrojAktivnihServisa: %w", err)
	}
	return n, nil
}

func (r *sqliteIzvestajRepo) PrihodTekuciMesec(ctx context.Context) (float64, error) {
	var iznos float64
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(
			(SELECT SUM(ukupno) FROM prodajni_nalozi
			 WHERE stornirano = 0
			   AND substr(datum, 1, 7) = strftime('%Y-%m', 'now', 'localtime')),
			0)
		+ COALESCE(
			(SELECT SUM(naplaceno + COALESCE(avans, 0)) FROM servisni_nalozi
			 WHERE status = 'Preuzeto'
			   AND (naplaceno > 0 OR avans > 0)
			   AND substr(datum_zavrsetka, 1, 7) = strftime('%Y-%m', 'now', 'localtime')),
			0)`).Scan(&iznos)
	if err != nil {
		return 0, fmt.Errorf("ntech: izvestaj.PrihodTekuciMesec: %w", err)
	}
	return iznos, nil
}

func (r *sqliteIzvestajRepo) BrojKriticnihZaliha(ctx context.Context) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM artikli WHERE kolicina <= kolicina_min`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("ntech: izvestaj.BrojKriticnihZaliha: %w", err)
	}
	return n, nil
}

func (r *sqliteIzvestajRepo) PoslednjiServisi(ctx context.Context, limit int) ([]model.ServisRedDashboard, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, uredjaj, status, datum_prijema FROM servisni_nalozi
		ORDER BY datum_prijema DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("ntech: izvestaj.PoslednjiServisi: %w", err)
	}
	defer rows.Close()
	var lista []model.ServisRedDashboard
	for rows.Next() {
		var s model.ServisRedDashboard
		if err := rows.Scan(&s.ID, &s.Uredjaj, &s.Status, &s.DatumPrijema); err != nil {
			return nil, fmt.Errorf("ntech: izvestaj.PoslednjiServisi: %w", err)
		}
		lista = append(lista, s)
	}
	return lista, rows.Err()
}

func (r *sqliteIzvestajRepo) KriticneZalihe(ctx context.Context, limit int) ([]model.ZalihaRed, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT naziv, kolicina, kolicina_min FROM artikli
		WHERE kolicina <= kolicina_min
		ORDER BY kolicina ASC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("ntech: izvestaj.KriticneZalihe: %w", err)
	}
	defer rows.Close()
	var lista []model.ZalihaRed
	for rows.Next() {
		var z model.ZalihaRed
		if err := rows.Scan(&z.Naziv, &z.Kolicina, &z.KolicinaMin); err != nil {
			return nil, fmt.Errorf("ntech: izvestaj.KriticneZalihe: %w", err)
		}
		lista = append(lista, z)
	}
	return lista, rows.Err()
}

func (r *sqliteIzvestajRepo) PoslednjeProdaje(ctx context.Context, limit int) ([]model.ProdajaRedDashboard, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			pn.broj_naloga, pn.ukupno, pn.datum,
			COALESCE(kp.naziv, '') AS klijent_naziv
		FROM prodajni_nalozi pn
		LEFT JOIN klijent_prikaz kp ON kp.id = pn.klijent_id
		WHERE pn.stornirano = 0
		ORDER BY pn.datum DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("ntech: izvestaj.PoslednjeProdaje: %w", err)
	}
	defer rows.Close()
	var lista []model.ProdajaRedDashboard
	for rows.Next() {
		var p model.ProdajaRedDashboard
		if err := rows.Scan(&p.BrojNaloga, &p.Ukupno, &p.Datum, &p.KlijentNaziv); err != nil {
			return nil, fmt.Errorf("ntech: izvestaj.PoslednjeProdaje: %w", err)
		}
		lista = append(lista, p)
	}
	return lista, rows.Err()
}

// mesecniPrihod je deljeni čitalac grupisanih (mesec, iznos) redova
func (r *sqliteIzvestajRepo) mesecniPrihod(ctx context.Context, upit, ime string) ([]model.MesecniIznos, error) {
	rows, err := r.db.QueryContext(ctx, upit)
	if err != nil {
		return nil, fmt.Errorf("ntech: izvestaj.%s: %w", ime, err)
	}
	defer rows.Close()
	var lista []model.MesecniIznos
	for rows.Next() {
		var m model.MesecniIznos
		if err := rows.Scan(&m.Mesec, &m.Iznos); err != nil {
			return nil, fmt.Errorf("ntech: izvestaj.%s: %w", ime, err)
		}
		lista = append(lista, m)
	}
	return lista, rows.Err()
}

func (r *sqliteIzvestajRepo) MesecniPrihodProdaja(ctx context.Context) ([]model.MesecniIznos, error) {
	return r.mesecniPrihod(ctx, `
		SELECT substr(datum, 1, 7), SUM(ukupno)
		FROM prodajni_nalozi
		WHERE stornirano = 0
		  AND substr(datum, 1, 10) >= date('now', '-11 months', 'start of month')
		GROUP BY substr(datum, 1, 7)`, "MesecniPrihodProdaja")
}

func (r *sqliteIzvestajRepo) MesecniPrihodServis(ctx context.Context) ([]model.MesecniIznos, error) {
	return r.mesecniPrihod(ctx, `
		SELECT substr(datum_zavrsetka, 1, 7), SUM(naplaceno + COALESCE(avans, 0))
		FROM servisni_nalozi
		WHERE datum_zavrsetka IS NOT NULL
		  AND status = 'Preuzeto'
		  AND (naplaceno > 0 OR avans > 0)
		  AND substr(datum_zavrsetka, 1, 10) >= date('now', '-11 months', 'start of month')
		GROUP BY substr(datum_zavrsetka, 1, 7)`, "MesecniPrihodServis")
}

func (r *sqliteIzvestajRepo) StariOtvoreniNalozi(ctx context.Context) ([]model.StariNalogRed, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT sn.id, sn.broj_naloga, sn.uredjaj, sn.status, sn.datum_prijema,
		       COALESCE(NULLIF(kp.naziv, ''), '—') AS klijent_naziv
		FROM servisni_nalozi sn
		LEFT JOIN klijent_prikaz kp ON kp.id = sn.klijent_id
		WHERE sn.datum_zavrsetka IS NULL
		  AND substr(sn.datum_prijema, 1, 10) <= date('now', '-14 days')
		ORDER BY sn.datum_prijema ASC`)
	if err != nil {
		return nil, fmt.Errorf("ntech: izvestaj.StariOtvoreniNalozi: %w", err)
	}
	defer rows.Close()
	var lista []model.StariNalogRed
	for rows.Next() {
		var sn model.StariNalogRed
		if err := rows.Scan(&sn.ID, &sn.BrojNaloga, &sn.Uredjaj, &sn.Status, &sn.DatumPrijema, &sn.KlijentNaziv); err != nil {
			return nil, fmt.Errorf("ntech: izvestaj.StariOtvoreniNalozi: %w", err)
		}
		lista = append(lista, sn)
	}
	return lista, rows.Err()
}

func (r *sqliteIzvestajRepo) TopArtikli(ctx context.Context, limit int) ([]model.TopArtikalRed, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT a.naziv, COALESCE(k.naziv, '—'), SUM(sp.kolicina), SUM(sp.ukupno)
		FROM stavke_prodaje sp
		JOIN artikli a ON a.id = sp.artikal_id
		LEFT JOIN kategorije k ON k.id = a.kategorija_id
		GROUP BY sp.artikal_id
		ORDER BY SUM(sp.kolicina) DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("ntech: izvestaj.TopArtikli: %w", err)
	}
	defer rows.Close()
	var lista []model.TopArtikalRed
	for rows.Next() {
		var a model.TopArtikalRed
		if err := rows.Scan(&a.Naziv, &a.Kategorija, &a.UkupnoKolicina, &a.UkupnoPrihod); err != nil {
			return nil, fmt.Errorf("ntech: izvestaj.TopArtikli: %w", err)
		}
		lista = append(lista, a)
	}
	return lista, rows.Err()
}

func (r *sqliteIzvestajRepo) TopKlijenti(ctx context.Context, limit int) ([]model.TopKlijentRed, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
		    kp.naziv AS naziv,
		    COALESCE(p.ukupno_prodaja, 0) + COALESCE(s.ukupno_servis, 0) AS ukupno_vrednost,
		    COALESCE(p.broj_prodaja, 0) + COALESCE(s.broj_servisa, 0) AS broj_naloga
		FROM klijenti k
		LEFT JOIN klijent_prikaz kp ON kp.id = k.id
		LEFT JOIN (
		    SELECT klijent_id, SUM(ukupno) AS ukupno_prodaja, COUNT(*) AS broj_prodaja
		    FROM prodajni_nalozi WHERE stornirano = 0 GROUP BY klijent_id
		) p ON p.klijent_id = k.id
		LEFT JOIN (
		    SELECT klijent_id, SUM(naplaceno + COALESCE(avans, 0)) AS ukupno_servis, COUNT(*) AS broj_servisa
		    FROM servisni_nalozi WHERE status = 'Preuzeto' AND (naplaceno > 0 OR avans > 0) GROUP BY klijent_id
		) s ON s.klijent_id = k.id
		WHERE COALESCE(p.ukupno_prodaja, 0) + COALESCE(s.ukupno_servis, 0) > 0
		ORDER BY ukupno_vrednost DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("ntech: izvestaj.TopKlijenti: %w", err)
	}
	defer rows.Close()
	var lista []model.TopKlijentRed
	for rows.Next() {
		var k model.TopKlijentRed
		if err := rows.Scan(&k.Naziv, &k.UkupnoVrednost, &k.BrojNaloga); err != nil {
			return nil, fmt.Errorf("ntech: izvestaj.TopKlijenti: %w", err)
		}
		lista = append(lista, k)
	}
	return lista, rows.Err()
}

// PrometniList vraća sve magacinske promene u zadatom periodu
func (r *sqliteIzvestajRepo) PrometniList(ctx context.Context, od, do time.Time) ([]model.PrometniRed, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT mp.datum, a.naziv, COALESCE(a.sifra, ''), mp.tip_promene,
		       mp.promena_kolicine, mp.stanje_pre, mp.stanje_posle,
		       COALESCE(mp.napomena, '')
		FROM magacinske_promene mp
		JOIN artikli a ON a.id = mp.artikal_id
		WHERE DATE(mp.datum) >= DATE(?) AND DATE(mp.datum) <= DATE(?)
		ORDER BY mp.datum ASC`,
		od.Format("2006-01-02"), do.Format("2006-01-02"),
	)
	if err != nil {
		return nil, fmt.Errorf("ntech: IzvestajRepo.PrometniList: %w", err)
	}
	defer rows.Close()

	var lista []model.PrometniRed
	for rows.Next() {
		var p model.PrometniRed
		if err := rows.Scan(&p.Datum, &p.ArtikalNaziv, &p.ArtikalSifra, &p.TipPromene,
			&p.PromenaKolicine, &p.StanjePre, &p.StanjePosle, &p.Napomena); err != nil {
			return nil, fmt.Errorf("ntech: IzvestajRepo.PrometniList: scan: %w", err)
		}
		lista = append(lista, p)
	}
	return lista, rows.Err()
}

// StanjeZaliha vraća trenutno stanje svih artikala sa vrednostima
func (r *sqliteIzvestajRepo) StanjeZaliha(ctx context.Context) ([]model.StanjeZalihaRed, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT a.naziv, COALESCE(a.sifra, ''), COALESCE(k.naziv, ''),
		       a.kolicina, a.kolicina_min, a.nabavna_cena, a.prodajna_cena,
		       a.cena_sa_pdv,
		       a.kolicina * a.nabavna_cena AS vrednost,
		       a.kolicina * a.cena_sa_pdv AS vrednost_sa_pdv
		FROM artikli a
		LEFT JOIN kategorije k ON k.id = a.kategorija_id
		ORDER BY k.naziv ASC, a.naziv ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("ntech: IzvestajRepo.StanjeZaliha: %w", err)
	}
	defer rows.Close()

	var lista []model.StanjeZalihaRed
	for rows.Next() {
		var s model.StanjeZalihaRed
		if err := rows.Scan(&s.Naziv, &s.Sifra, &s.Kategorija,
			&s.Kolicina, &s.KolicinMin, &s.NabavnaCena, &s.ProdajnaCena,
			&s.CenaSaPdv, &s.VrednostZalihe, &s.VrednostSaPdv); err != nil {
			return nil, fmt.Errorf("ntech: IzvestajRepo.StanjeZaliha: scan: %w", err)
		}
		lista = append(lista, s)
	}
	return lista, rows.Err()
}
