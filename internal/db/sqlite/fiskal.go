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
// ProdajaID je UNIQUE — drugi poziv sa istim ID-jem pada na constraint.
func (r *FiskalRepo) Kreiraj(ctx context.Context, fr *model.FiskalniRacun) (int64, error) {
	rez, err := r.db.ExecContext(ctx, `
		INSERT INTO fiskalni_racuni (
			prodaja_id, tip_racuna, tip_transakcije, pfr_broj, pfr_vreme,
			brojac, ekstenzija_brojaca, url_verifikacija, qr_kod,
			poreske_stavke, ukupno_za_naplatu, ukupan_porez,
			sirovi_odgovor, potpisao, zatrazio, poruka
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, fr.ProdajaID, fr.TipRacuna, fr.TipTransakcije, fr.PfrBroj, fr.PfrVreme,
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
// Vraća nil bez greške ako račun ne postoji (nije fiskalizovano).
func (r *FiskalRepo) DohvatiPoProdaji(ctx context.Context, prodajaID int64) (*model.FiskalniRacun, error) {
	fr := &model.FiskalniRacun{}
	var storniran int
	err := r.db.QueryRowContext(ctx, `
		SELECT id, prodaja_id, tip_racuna, tip_transakcije, pfr_broj, pfr_vreme,
			   brojac, ekstenzija_brojaca, url_verifikacija, qr_kod,
			   poreske_stavke, ukupno_za_naplatu, ukupan_porez,
			   sirovi_odgovor, potpisao, zatrazio, poruka, storniran, vreme_kreiranja
		FROM fiskalni_racuni WHERE prodaja_id = ?
	`, prodajaID).Scan(
		&fr.ID, &fr.ProdajaID, &fr.TipRacuna, &fr.TipTransakcije, &fr.PfrBroj, &fr.PfrVreme,
		&fr.Brojac, &fr.EkstenzijaBrojaca, &fr.UrlVerifikacija, &fr.QRKod,
		&fr.PoreskeStavke, &fr.UkupnoZaNaplatu, &fr.UkupanPorez,
		&fr.SiroviOdgovor, &fr.Potpisao, &fr.Zatrazio, &fr.Poruka, &storniran, &fr.VremeKreiranja,
	)
	if err == sql.ErrNoRows {
		return nil, nil // nije fiskalizovano — nije greška
	}
	if err != nil {
		return nil, fmt.Errorf("ntech: FiskalRepo.DohvatiPoProdaji: %w", err)
	}
	fr.Storniran = storniran != 0
	return fr, nil
}
