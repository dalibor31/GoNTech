package sqlite

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"ntech/internal/model"
)

// generisiJavniToken kreira 32-znakovni hex token za javni URL
func generisiJavniToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ServisRepo je SQLite implementacija ServisRepository interfejsa
type ServisRepo struct {
	db *sql.DB
}

// NoviServisRepo kreira novi ServisRepo
func NoviServisRepo(db *sql.DB) *ServisRepo {
	return &ServisRepo{db: db}
}

// SledeciBroj generiše sledeći broj naloga u formatu SN-MMGG-NNN
// (MM mesec, GG dvocifrena godina); brojač NNN se resetuje svakog meseca
func (r *ServisRepo) SledeciBroj(ctx context.Context) (string, error) {
	sada := time.Now()
	// prefiks "SN-MMGG-" je dug 8 karaktera, pa brojač počinje od 9. karaktera
	prefiks := fmt.Sprintf("SN-%02d%02d-", int(sada.Month()), sada.Year()%100)
	uzorak := prefiks + "%"

	// COALESCE(MAX, 0)+1 → prvi nalog u mesecu dobija 001
	var sledeci int
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(CAST(SUBSTR(broj_naloga, 9) AS INTEGER)), 0) + 1
		FROM servisni_nalozi
		WHERE broj_naloga LIKE ?`, uzorak).Scan(&sledeci)
	if err != nil {
		return "", fmt.Errorf("ntech: ServisRepo.SledeciBroj: %w", err)
	}

	return fmt.Sprintf("%s%03d", prefiks, sledeci), nil
}

// Lista vraća listu servisnih naloga sa imenom klijenta, opcionim filterima
func (r *ServisRepo) Lista(ctx context.Context, pretraga, status string) ([]model.ServisniNalogSaKlijentom, error) {
	upit := `
		SELECT
			sn.id, sn.klijent_id, sn.tehnicar_id, sn.broj_naloga, sn.uredjaj, sn.serijski_broj,
			sn.opis_kvara, sn.trazene_nadogradnje, sn.status, sn.cena_od, sn.cena_do, sn.cena_konacna,
			sn.avans, sn.napomena, sn.garancija_do, sn.datum_prijema, sn.datum_zavrsetka, sn.predvidjen_datum,
			sn.ostecenja, sn.pin_uredjaja, sn.pribor, sn.napomena_klijentu, sn.javni_token,
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
			opis_kvara, trazene_nadogradnje, status, cena_od, cena_do, cena_konacna,
			avans, napomena, garancija_do, datum_prijema, datum_zavrsetka, predvidjen_datum,
			ostecenja, pin_uredjaja, pribor, napomena_klijentu, javni_token
		FROM servisni_nalozi WHERE id = ?`, id)

	var n model.ServisniNalog
	err := scanNalog(red.Scan, &n, nil)
	if err != nil {
		return nil, fmt.Errorf("ntech: ServisRepo.DohvatiID: %w", err)
	}

	return &n, nil
}

// Kreiraj upisuje novi servisni nalog u bazu i generiše javni token
func (r *ServisRepo) Kreiraj(ctx context.Context, n *model.ServisniNalog) (int64, error) {
	token, err := generisiJavniToken()
	if err != nil {
		return 0, fmt.Errorf("ntech: ServisRepo.Kreiraj: token: %w", err)
	}

	rezultat, err := r.db.ExecContext(ctx, `
		INSERT INTO servisni_nalozi
			(klijent_id, tehnicar_id, broj_naloga, uredjaj, serijski_broj, opis_kvara, trazene_nadogradnje,
			 status, cena_od, cena_do, cena_konacna, avans, napomena, garancija_do, datum_zavrsetka, predvidjen_datum,
			 ostecenja, pin_uredjaja, pribor, datum_prijema, javni_token)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		nullInt64(n.KlijentID), nullInt64(n.TehnicarID), n.BrojNaloga, n.Uredjaj,
		nullString(n.SerijskiBroj), n.OpisKvara, n.TrazeneNadogradnje, n.Status,
		nullFloat64(n.CenaOd), nullFloat64(n.CenaDo), nullFloat64(n.CenaKonacna),
		nullFloat64(n.Avans), nullString(n.Napomena),
		nullTime(n.GarancijaDo), nullTime(n.DatumZavrsetka), nullTime(n.PredvidjenDatum),
		nullString(n.Ostecenja), nullString(n.PinUredjaja), nullString(n.Pribor),
		n.DatumPrijema, token,
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

// DohvatiJavniToken vraća servisni nalog po javnom tokenu — bez autentifikacije
func (r *ServisRepo) DohvatiJavniToken(ctx context.Context, token string) (*model.ServisniNalog, error) {
	red := r.db.QueryRowContext(ctx, `
		SELECT
			id, klijent_id, tehnicar_id, broj_naloga, uredjaj, serijski_broj,
			opis_kvara, trazene_nadogradnje, status, cena_od, cena_do, cena_konacna,
			avans, napomena, garancija_do, datum_prijema, datum_zavrsetka, predvidjen_datum,
			ostecenja, pin_uredjaja, pribor, napomena_klijentu, javni_token
		FROM servisni_nalozi WHERE javni_token = ?`, token)

	var n model.ServisniNalog
	if err := scanNalog(red.Scan, &n, nil); err != nil {
		return nil, fmt.Errorf("ntech: ServisRepo.DohvatiJavniToken: %w", err)
	}
	return &n, nil
}

// Izmeni ažurira postojeći servisni nalog — broj_naloga i datum_prijema se ne menjaju
func (r *ServisRepo) Izmeni(ctx context.Context, n *model.ServisniNalog) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE servisni_nalozi SET
			klijent_id = ?, tehnicar_id = ?, uredjaj = ?, serijski_broj = ?, opis_kvara = ?, trazene_nadogradnje = ?,
			status = ?, cena_od = ?, cena_do = ?, cena_konacna = ?,
			avans = ?, napomena = ?, garancija_do = ?, datum_zavrsetka = ?, predvidjen_datum = ?,
			ostecenja = ?, pin_uredjaja = ?, pribor = ?
		WHERE id = ?`,
		nullInt64(n.KlijentID), nullInt64(n.TehnicarID), n.Uredjaj, nullString(n.SerijskiBroj), n.OpisKvara, n.TrazeneNadogradnje,
		n.Status, nullFloat64(n.CenaOd), nullFloat64(n.CenaDo), nullFloat64(n.CenaKonacna),
		nullFloat64(n.Avans), nullString(n.Napomena), nullTime(n.GarancijaDo), nullTime(n.DatumZavrsetka), nullTime(n.PredvidjenDatum),
		nullString(n.Ostecenja), nullString(n.PinUredjaja), nullString(n.Pribor),
		n.ID,
	)
	if err != nil {
		return fmt.Errorf("ntech: ServisRepo.Izmeni: %w", err)
	}

	return nil
}

// AzurirajStatus menja samo status naloga; ako nalog prelazi u završno stanje
// i datum_zavrsetka još nije postavljen, automatski ga postavlja na danas.
func (r *ServisRepo) AzurirajStatus(ctx context.Context, id int64, status string) error {
	var upit string
	if status == model.StatusZavrseno || status == model.StatusPreuzeto {
		upit = `UPDATE servisni_nalozi SET status = ?,
			datum_zavrsetka = COALESCE(datum_zavrsetka, date('now', 'localtime'))
			WHERE id = ?`
	} else {
		upit = `UPDATE servisni_nalozi SET status = ? WHERE id = ?`
	}
	_, err := r.db.ExecContext(ctx, upit, status, id)
	if err != nil {
		return fmt.Errorf("ntech: ServisRepo.AzurirajStatus: %w", err)
	}
	return nil
}

// AzurirajGaranciju postavlja ili briše datum garancije na servisnom nalogu.
// garancijaDo == nil → bez garancije.
func (r *ServisRepo) AzurirajGaranciju(ctx context.Context, id int64, garancijaDo *time.Time) error {
	if garancijaDo != nil {
		_, err := r.db.ExecContext(ctx,
			"UPDATE servisni_nalozi SET garancija_do = ? WHERE id = ?",
			garancijaDo.Format("2006-01-02"), id,
		)
		if err != nil {
			return fmt.Errorf("ntech: ServisRepo.AzurirajGaranciju: %w", err)
		}
	} else {
		_, err := r.db.ExecContext(ctx,
			"UPDATE servisni_nalozi SET garancija_do = NULL WHERE id = ?", id,
		)
		if err != nil {
			return fmt.Errorf("ntech: ServisRepo.AzurirajGaranciju: %w", err)
		}
	}
	return nil
}

// AzurirajPredvidjenDatum postavlja ili briše ručni override predviđenog datuma popravke.
// predvidjenDatum == nil → vraća se na izvedeni default (prijem + rok iz podešavanja).
func (r *ServisRepo) AzurirajPredvidjenDatum(ctx context.Context, id int64, predvidjenDatum *time.Time) error {
	if predvidjenDatum != nil {
		_, err := r.db.ExecContext(ctx,
			"UPDATE servisni_nalozi SET predvidjen_datum = ? WHERE id = ?",
			predvidjenDatum.Format("2006-01-02"), id,
		)
		if err != nil {
			return fmt.Errorf("ntech: ServisRepo.AzurirajPredvidjenDatum: %w", err)
		}
	} else {
		_, err := r.db.ExecContext(ctx,
			"UPDATE servisni_nalozi SET predvidjen_datum = NULL WHERE id = ?", id,
		)
		if err != nil {
			return fmt.Errorf("ntech: ServisRepo.AzurirajPredvidjenDatum: %w", err)
		}
	}
	return nil
}

// AzurirajTehnicar postavlja ili uklanja dodeljenog tehničara na nalogu.
// tehnicarID == nil → nedodeljen.
func (r *ServisRepo) AzurirajTehnicar(ctx context.Context, id int64, tehnicarID *int64) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE servisni_nalozi SET tehnicar_id = ? WHERE id = ?",
		nullInt64(tehnicarID), id,
	)
	if err != nil {
		return fmt.Errorf("ntech: ServisRepo.AzurirajTehnicar: %w", err)
	}
	return nil
}

// AzurirajNapomenuKlijentu postavlja tekst napomene namenjene klijentu na nalogu
func (r *ServisRepo) AzurirajNapomenuKlijentu(ctx context.Context, id int64, tekst string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE servisni_nalozi SET napomena_klijentu = ? WHERE id = ?", tekst, id,
	)
	if err != nil {
		return fmt.Errorf("ntech: ServisRepo.AzurirajNapomenuKlijentu: %w", err)
	}
	return nil
}

// Obrisi briše servisni nalog po ID-u
// Obrisi briše servisni nalog i vraća ugrađene delove na stanje u magacinu (u transakciji)
func (r *ServisRepo) Obrisi(ctx context.Context, id int64, korisnikID *int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ntech: ServisRepo.Obrisi: begin tx: %w", err)
	}
	defer tx.Rollback()

	// pokupi ugrađene delove pre brisanja (CASCADE bi ih obrisao bez povraćaja)
	redovi, err := tx.QueryContext(ctx,
		"SELECT artikal_id, kolicina FROM servisni_delovi WHERE nalog_id = ?", id)
	if err != nil {
		return fmt.Errorf("ntech: ServisRepo.Obrisi: dohvati delove: %w", err)
	}
	type povrat struct {
		artikalID int64
		kolicina  int
	}
	var delovi []povrat
	for redovi.Next() {
		var p povrat
		if err := redovi.Scan(&p.artikalID, &p.kolicina); err != nil {
			redovi.Close()
			return fmt.Errorf("ntech: ServisRepo.Obrisi: scan dela: %w", err)
		}
		delovi = append(delovi, p)
	}
	redovi.Close()

	for _, p := range delovi {
		// usluge i troškovi nemaju stanje na lageru — vraćamo samo proizvodima
		var stanjePre int
		var tip string
		err := tx.QueryRowContext(ctx,
			"SELECT kolicina, tip FROM artikli WHERE id = ?", p.artikalID,
		).Scan(&stanjePre, &tip)
		if err != nil {
			return fmt.Errorf("ntech: ServisRepo.Obrisi: dohvati stanje: %w", err)
		}
		if !(tip == model.TipProizvod || tip == "") {
			continue
		}

		stanjePosle := stanjePre + p.kolicina
		_, err = tx.ExecContext(ctx,
			"UPDATE artikli SET kolicina = ? WHERE id = ?", stanjePosle, p.artikalID,
		)
		if err != nil {
			return fmt.Errorf("ntech: ServisRepo.Obrisi: vrati stanje: %w", err)
		}

		err = zabeleziMagacinPromenu(ctx, tx, p.artikalID, model.PromenaPovracaj,
			p.kolicina, stanjePre, stanjePosle, id, korisnikID, "brisanje servisnog naloga")
		if err != nil {
			return fmt.Errorf("ntech: ServisRepo.Obrisi: magacin: %w", err)
		}
	}

	// potraživani delovi nemaju ON DELETE CASCADE — ručno ih čistimo da brisanje ne padne na FK
	if _, err := tx.ExecContext(ctx,
		"DELETE FROM servisni_potrazivani_delovi WHERE nalog_id = ?", id); err != nil {
		return fmt.Errorf("ntech: ServisRepo.Obrisi: potraživani: %w", err)
	}

	// CASCADE briše servisni_delovi i servisni_radovi
	if _, err := tx.ExecContext(ctx, "DELETE FROM servisni_nalozi WHERE id = ?", id); err != nil {
		return fmt.Errorf("ntech: ServisRepo.Obrisi: delete: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("ntech: ServisRepo.Obrisi: commit: %w", err)
	}

	return nil
}

// scanNalog čita redove iz upita u ServisniNalog struct —
// klijentNaziv je opcioni pokazivač, nil kada se čita bez JOIN-a
func scanNalog(scan func(...any) error, n *model.ServisniNalog, klijentNaziv *string) error {
	var klijentID, tehnicarID sql.NullInt64
	var serijskiBroj, napomena, ostecenja, pinUredjaja, pribor, napomenaKlijentu, javniToken sql.NullString
	var cenaOd, cenaDo, cenaKonacna, avans sql.NullFloat64
	var garancijaDo, datumZavrsetka, predvidjenDatum sql.NullTime

	args := []any{
		&n.ID, &klijentID, &tehnicarID, &n.BrojNaloga, &n.Uredjaj, &serijskiBroj,
		&n.OpisKvara, &n.TrazeneNadogradnje, &n.Status, &cenaOd, &cenaDo, &cenaKonacna,
		&avans, &napomena, &garancijaDo, &n.DatumPrijema, &datumZavrsetka, &predvidjenDatum,
		&ostecenja, &pinUredjaja, &pribor, &napomenaKlijentu, &javniToken,
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
	n.NapomenaKlijentu = napomenaKlijentu.String
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
	if predvidjenDatum.Valid {
		v := predvidjenDatum.Time
		n.PredvidjenDatum = &v
	}
	n.JavniToken = javniToken.String

	return nil
}
