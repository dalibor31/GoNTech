package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"ntech/internal/model"
)

// FiskalRepo je SQLite implementacija FiskalRepository interfejsa
type FiskalRepo struct {
	db *sql.DB
}

// NoviFiskalRepo kreira novi FiskalRepo
func NoviFiskalRepo(db *sql.DB) *FiskalRepo {
	return &FiskalRepo{db: db}
}

// Kreiraj dodaje novi fiskalni račun u bazu. Vraća ID kreiranog reda.
// Koristi prodaja_id (za prodaju) ILI servis_id (za servis) — ne oba.
func (r *FiskalRepo) Kreiraj(ctx context.Context, fr *model.FiskalniRacun) (int64, error) {
	var prodajaID, servisID any
	if fr.ProdajaID > 0 {
		prodajaID = fr.ProdajaID
	}
	if fr.ServisID > 0 {
		servisID = fr.ServisID
	}
	rez, err := r.db.ExecContext(ctx, `
		INSERT INTO fiskalni_racuni (
			prodaja_id, servis_id, tip_racuna, tip_transakcije, pfr_broj, pfr_vreme,
			brojac, ekstenzija_brojaca, url_verifikacija, qr_kod,
			poreske_stavke, ukupno_za_naplatu, ukupan_porez,
			sirovi_odgovor, potpisao, zatrazio, poruka
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, prodajaID, servisID, fr.TipRacuna, fr.TipTransakcije, fr.PfrBroj, fr.PfrVreme,
		fr.Brojac, fr.EkstenzijaBrojaca, fr.UrlVerifikacija, fr.QRKod,
		fr.PoreskeStavke, fr.UkupnoZaNaplatu, fr.UkupanPorez,
		fr.SiroviOdgovor, fr.Potpisao, fr.Zatrazio, fr.Poruka,
	)
	if err != nil {
		return 0, fmt.Errorf("ntech: FiskalRepo.Kreiraj: %w", err)
	}
	id, err := rez.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("ntech: FiskalRepo.Kreiraj.LastInsertId: %w", err)
	}
	return id, nil
}

// DohvatiPoProdaji vraća fiskalni račun za dati prodajni nalog.
// Vraća nil bez greške ako račun ne postoji.
func (r *FiskalRepo) DohvatiPoProdaji(ctx context.Context, prodajaID int64) (*model.FiskalniRacun, error) {
	fr := &model.FiskalniRacun{}
	var storniran int
	var sID sql.NullInt64
	err := r.db.QueryRowContext(ctx, `
		SELECT id, prodaja_id, servis_id, tip_racuna, tip_transakcije, pfr_broj, pfr_vreme,
			   brojac, ekstenzija_brojaca, url_verifikacija, qr_kod,
			   poreske_stavke, ukupno_za_naplatu, ukupan_porez,
			   sirovi_odgovor, potpisao, zatrazio, poruka, storniran, vreme_kreiranja
		FROM fiskalni_racuni WHERE prodaja_id = ?
		ORDER BY id DESC LIMIT 1
	`, prodajaID).Scan(
		&fr.ID, &fr.ProdajaID, &sID, &fr.TipRacuna, &fr.TipTransakcije, &fr.PfrBroj, &fr.PfrVreme,
		&fr.Brojac, &fr.EkstenzijaBrojaca, &fr.UrlVerifikacija, &fr.QRKod,
		&fr.PoreskeStavke, &fr.UkupnoZaNaplatu, &fr.UkupanPorez,
		&fr.SiroviOdgovor, &fr.Potpisao, &fr.Zatrazio, &fr.Poruka, &storniran, &fr.VremeKreiranja,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ntech: FiskalRepo.DohvatiPoProdaji: %w", err)
	}
	if sID.Valid {
		fr.ServisID = sID.Int64
	}
	fr.Storniran = storniran != 0
	return fr, nil
}

// DohvatiPoServisu vraća fiskalni račun za dati servisni nalog.
// Vraća nil bez greške ako račun ne postoji.
func (r *FiskalRepo) DohvatiPoServisu(ctx context.Context, servisID int64) (*model.FiskalniRacun, error) {
	fr := &model.FiskalniRacun{}
	var storniran int
	var sID, pID sql.NullInt64
	err := r.db.QueryRowContext(ctx, `
		SELECT id, prodaja_id, servis_id, tip_racuna, tip_transakcije, pfr_broj, pfr_vreme,
			   brojac, ekstenzija_brojaca, url_verifikacija, qr_kod,
			   poreske_stavke, ukupno_za_naplatu, ukupan_porez,
			   sirovi_odgovor, potpisao, zatrazio, poruka, storniran, vreme_kreiranja
		FROM fiskalni_racuni WHERE servis_id = ?
		ORDER BY id DESC LIMIT 1
	`, servisID).Scan(
		&fr.ID, &pID, &sID, &fr.TipRacuna, &fr.TipTransakcije, &fr.PfrBroj, &fr.PfrVreme,
		&fr.Brojac, &fr.EkstenzijaBrojaca, &fr.UrlVerifikacija, &fr.QRKod,
		&fr.PoreskeStavke, &fr.UkupnoZaNaplatu, &fr.UkupanPorez,
		&fr.SiroviOdgovor, &fr.Potpisao, &fr.Zatrazio, &fr.Poruka, &storniran, &fr.VremeKreiranja,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ntech: FiskalRepo.DohvatiPoServisu: %w", err)
	}
	if pID.Valid {
		fr.ProdajaID = pID.Int64
	}
	if sID.Valid {
		fr.ServisID = sID.Int64
	}
	fr.Storniran = storniran != 0
	return fr, nil
}

// DohvatiPoServisuITip vraća poslednji nestorniran fiskalni račun za dati servisni
// nalog sa tačno određenim tipom (npr. tip_racuna="Advance", tip_transakcije="Sale"
// za avansni račun) — za razliku od DohvatiPoServisu koji vraća samo najnoviji
// zapis bilo kog tipa. Vraća nil bez greške ako takav račun ne postoji.
func (r *FiskalRepo) DohvatiPoServisuITip(ctx context.Context, servisID int64, tipRacuna, tipTransakcije string) (*model.FiskalniRacun, error) {
	fr := &model.FiskalniRacun{}
	var storniran int
	var sID, pID sql.NullInt64
	err := r.db.QueryRowContext(ctx, `
		SELECT id, prodaja_id, servis_id, tip_racuna, tip_transakcije, pfr_broj, pfr_vreme,
			   brojac, ekstenzija_brojaca, url_verifikacija, qr_kod,
			   poreske_stavke, ukupno_za_naplatu, ukupan_porez,
			   sirovi_odgovor, potpisao, zatrazio, poruka, storniran, vreme_kreiranja
		FROM fiskalni_racuni
		WHERE servis_id = ? AND tip_racuna = ? AND tip_transakcije = ? AND storniran = 0
		ORDER BY id DESC LIMIT 1
	`, servisID, tipRacuna, tipTransakcije).Scan(
		&fr.ID, &pID, &sID, &fr.TipRacuna, &fr.TipTransakcije, &fr.PfrBroj, &fr.PfrVreme,
		&fr.Brojac, &fr.EkstenzijaBrojaca, &fr.UrlVerifikacija, &fr.QRKod,
		&fr.PoreskeStavke, &fr.UkupnoZaNaplatu, &fr.UkupanPorez,
		&fr.SiroviOdgovor, &fr.Potpisao, &fr.Zatrazio, &fr.Poruka, &storniran, &fr.VremeKreiranja,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ntech: FiskalRepo.DohvatiPoServisuITip: %w", err)
	}
	if pID.Valid {
		fr.ProdajaID = pID.Int64
	}
	if sID.Valid {
		fr.ServisID = sID.Int64
	}
	fr.Storniran = storniran != 0
	return fr, nil
}

// SumaAvansaPoServisu vraća neto fiskalizovan avans za dati servisni nalog —
// zbir Advance/Sale iznosa umanjen za Advance/Refund iznose (nestornirani).
// Koristi se da se pri izmeni avansa fiskalizuje samo razlika (delta), ne ceo
// novi iznos ponovo.
func (r *FiskalRepo) SumaAvansaPoServisu(ctx context.Context, servisID int64) (float64, error) {
	var suma float64
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(CASE WHEN tip_transakcije = 'Refund' THEN -ukupno_za_naplatu ELSE ukupno_za_naplatu END), 0)
		FROM fiskalni_racuni
		WHERE servis_id = ? AND tip_racuna = 'Advance' AND storniran = 0
	`, servisID).Scan(&suma)
	if err != nil {
		return 0, fmt.Errorf("ntech: FiskalRepo.SumaAvansaPoServisu: %w", err)
	}
	return suma, nil
}

// ServisiBezFiskalnog vraća skup ID-eva servisnih naloga koji su u statusu "Preuzeto"
// a nemaju fiskalni račun (za oznaku u listi naloga).
func (r *FiskalRepo) ServisiBezFiskalnog(ctx context.Context) (map[int64]bool, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT sn.id FROM servisni_nalozi sn
		LEFT JOIN fiskalni_racuni fr ON fr.servis_id = sn.id AND fr.storniran = 0
		WHERE sn.status = 'Preuzeto' AND fr.id IS NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[int64]bool)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// ProdajeBezFiskalnog vraća skup ID-eva prodajnih naloga koji nisu stornirani
// a nemaju fiskalni račun (za oznaku u listi naloga).
func (r *FiskalRepo) ProdajeBezFiskalnog(ctx context.Context) (map[int64]bool, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT pn.id FROM prodajni_nalozi pn
		LEFT JOIN fiskalni_racuni fr ON fr.prodaja_id = pn.id AND fr.storniran = 0
		WHERE pn.stornirano = 0 AND fr.id IS NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[int64]bool)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// OznačiKaoStorniran postavlja storniran=1 za fiskalni račun sa datim ID-jem.
func (r *FiskalRepo) OznačiKaoStorniran(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "UPDATE fiskalni_racuni SET storniran = 1 WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("ntech: FiskalRepo.OznačiKaoStorniran: %w", err)
	}
	return nil
}
