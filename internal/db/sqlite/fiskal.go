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
