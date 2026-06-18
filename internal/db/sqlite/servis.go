package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"ntech/internal/model"
)

// ServisRepo je SQLite implementacija ServisRepository interfejsa
type ServisRepo struct {
	db *sql.DB
}

// NoviServisRepo kreira novi ServisRepo
func NoviServisRepo(db *sql.DB) *ServisRepo {
	return &ServisRepo{db: db}
}

// SledeciBroj generiše sledeći broj naloga u formatu SN-GGGG-NNNN
func (r *ServisRepo) SledeciBroj(ctx context.Context) (string, error) {
	godina := time.Now().Year()
	uzorak := fmt.Sprintf("SN-%d-%%", godina)

	var sledeci int
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(CAST(SUBSTR(broj_naloga, 9) AS INTEGER)), 0) + 1
		FROM servisni_nalozi
		WHERE broj_naloga LIKE ?`, uzorak).Scan(&sledeci)
	if err != nil {
		return "", fmt.Errorf("ntech: ServisRepo.SledeciBroj: %w", err)
	}

	return fmt.Sprintf("SN-%d-%04d", godina, sledeci), nil
}

// Lista vraća listu servisnih naloga sa imenom klijenta, opcionim filterima
func (r *ServisRepo) Lista(ctx context.Context, pretraga, status string) ([]model.ServisniNalogSaKlijentom, error) {
	upit := `
		SELECT
			sn.id, sn.klijent_id, sn.tehnicar_id, sn.broj_naloga, sn.uredjaj, sn.serijski_broj,
			sn.opis_kvara, sn.status, sn.cena_od, sn.cena_do, sn.cena_konacna,
			sn.avans, sn.napomena, sn.garancija_do, sn.datum_prijema, sn.datum_zavrsetka,
			sn.ostecenja, sn.pin_uredjaja, sn.pribor,
			COALESCE(kp.naziv, '') AS klijent_naziv
		FROM servisni_nalozi sn
		LEFT JOIN klijent_prikaz kp ON kp.id = sn.klijent_id
		WHERE 1=1`

	args := []any{}

	if pretraga != "" {
		upit += " AND (sn.broj_naloga LIKE ? OR sn.uredjaj LIKE ?)"
		p := "%" + pretraga + "%"
		args = append(args, p, p)
	}

	if status != "" {
		upit += " AND sn.status = ?"
		args = append(args, status)
	}

	upit += " ORDER BY sn.datum_prijema DESC"

	redovi, err := r.db.QueryContext(ctx, upit, args...)
	if err != nil {
		return nil, fmt.Errorf("ntech: ServisRepo.Lista: %w", err)
	}
	defer redovi.Close()

	var rezultat []model.ServisniNalogSaKlijentom
	for redovi.Next() {
		var n model.ServisniNalogSaKlijentom
		err := scanNalog(redovi.Scan, &n.ServisniNalog, &n.KlijentNaziv)
		if err != nil {
			return nil, fmt.Errorf("ntech: ServisRepo.Lista: scan: %w", err)
		}
		rezultat = append(rezultat, n)
	}

	return rezultat, nil
}

// DohvatiID vraća jedan servisni nalog po ID-u
func (r *ServisRepo) DohvatiID(ctx context.Context, id int64) (*model.ServisniNalog, error) {
	red := r.db.QueryRowContext(ctx, `
		SELECT
			id, klijent_id, tehnicar_id, broj_naloga, uredjaj, serijski_broj,
			opis_kvara, status, cena_od, cena_do, cena_konacna,
			avans, napomena, garancija_do, datum_prijema, datum_zavrsetka,
			ostecenja, pin_uredjaja, pribor
		FROM servisni_nalozi WHERE id = ?`, id)

	var n model.ServisniNalog
	err := scanNalog(red.Scan, &n, nil)
	if err != nil {
		return nil, fmt.Errorf("ntech: ServisRepo.DohvatiID: %w", err)
	}

	return &n, nil
}

// Kreiraj upisuje novi servisni nalog u bazu
func (r *ServisRepo) Kreiraj(ctx context.Context, n *model.ServisniNalog) (int64, error) {
	rezultat, err := r.db.ExecContext(ctx, `
		INSERT INTO servisni_nalozi
			(klijent_id, tehnicar_id, broj_naloga, uredjaj, serijski_broj, opis_kvara,
			 status, cena_od, cena_do, cena_konacna, avans, napomena, garancija_do, datum_zavrsetka,
			 ostecenja, pin_uredjaja, pribor, datum_prijema)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		nullInt64(n.KlijentID), nullInt64(n.TehnicarID), n.BrojNaloga, n.Uredjaj,
		nullString(n.SerijskiBroj), n.OpisKvara, n.Status,
		nullFloat64(n.CenaOd), nullFloat64(n.CenaDo), nullFloat64(n.CenaKonacna),
		nullFloat64(n.Avans), nullString(n.Napomena),
		nullTime(n.GarancijaDo), nullTime(n.DatumZavrsetka),
		nullString(n.Ostecenja), nullString(n.PinUredjaja), nullString(n.Pribor),
		n.DatumPrijema,
	)
	if err != nil {
		return 0, fmt.Errorf("ntech: ServisRepo.Kreiraj: %w", err)
	}

	id, err := rezultat.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("ntech: ServisRepo.Kreiraj: last insert id: %w", err)
	}

	return id, nil
}

// Izmeni ažurira postojeći servisni nalog — broj_naloga i datum_prijema se ne menjaju
func (r *ServisRepo) Izmeni(ctx context.Context, n *model.ServisniNalog) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE servisni_nalozi SET
			klijent_id = ?, tehnicar_id = ?, uredjaj = ?, serijski_broj = ?, opis_kvara = ?,
			status = ?, cena_od = ?, cena_do = ?, cena_konacna = ?,
			avans = ?, napomena = ?, garancija_do = ?, datum_zavrsetka = ?,
			ostecenja = ?, pin_uredjaja = ?, pribor = ?
		WHERE id = ?`,
		nullInt64(n.KlijentID), nullInt64(n.TehnicarID), n.Uredjaj, nullString(n.SerijskiBroj), n.OpisKvara,
		n.Status, nullFloat64(n.CenaOd), nullFloat64(n.CenaDo), nullFloat64(n.CenaKonacna),
		nullFloat64(n.Avans), nullString(n.Napomena), nullTime(n.GarancijaDo), nullTime(n.DatumZavrsetka),
		nullString(n.Ostecenja), nullString(n.PinUredjaja), nullString(n.Pribor),
		n.ID,
	)
	if err != nil {
		return fmt.Errorf("ntech: ServisRepo.Izmeni: %w", err)
	}

	return nil
}

// Obrisi briše servisni nalog po ID-u
func (r *ServisRepo) Obrisi(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM servisni_nalozi WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("ntech: ServisRepo.Obrisi: %w", err)
	}

	return nil
}

// scanNalog čita redove iz upita u ServisniNalog struct —
// klijentNaziv je opcioni pokazivač, nil kada se čita bez JOIN-a
func scanNalog(scan func(...any) error, n *model.ServisniNalog, klijentNaziv *string) error {
	var klijentID, tehnicarID sql.NullInt64
	var serijskiBroj, napomena, ostecenja, pinUredjaja, pribor sql.NullString
	var cenaOd, cenaDo, cenaKonacna, avans sql.NullFloat64
	var garancijaDo, datumZavrsetka sql.NullTime

	args := []any{
		&n.ID, &klijentID, &tehnicarID, &n.BrojNaloga, &n.Uredjaj, &serijskiBroj,
		&n.OpisKvara, &n.Status, &cenaOd, &cenaDo, &cenaKonacna,
		&avans, &napomena, &garancijaDo, &n.DatumPrijema, &datumZavrsetka,
		&ostecenja, &pinUredjaja, &pribor,
	}

	if klijentNaziv != nil {
		args = append(args, klijentNaziv)
	}

	if err := scan(args...); err != nil {
		return err
	}

	if klijentID.Valid {
		v := klijentID.Int64
		n.KlijentID = &v
	}
	if tehnicarID.Valid {
		v := tehnicarID.Int64
		n.TehnicarID = &v
	}
	n.SerijskiBroj = serijskiBroj.String
	n.Napomena = napomena.String
	n.Ostecenja = ostecenja.String
	n.PinUredjaja = pinUredjaja.String
	n.Pribor = pribor.String
	if cenaOd.Valid {
		v := cenaOd.Float64
		n.CenaOd = &v
	}
	if cenaDo.Valid {
		v := cenaDo.Float64
		n.CenaDo = &v
	}
	if cenaKonacna.Valid {
		v := cenaKonacna.Float64
		n.CenaKonacna = &v
	}
	if avans.Valid {
		v := avans.Float64
		n.Avans = &v
	}
	if garancijaDo.Valid {
		v := garancijaDo.Time
		n.GarancijaDo = &v
	}
	if datumZavrsetka.Valid {
		v := datumZavrsetka.Time
		n.DatumZavrsetka = &v
	}

	return nil
}
