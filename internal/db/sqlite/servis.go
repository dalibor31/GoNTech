package sqlite

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
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

// SledeciBroj generiše sledeći broj naloga u formatu SN-GGMM-NNN
// (GG dvocifrena godina, MM mesec); brojač NNN se resetuje svakog meseca.
// Koristi se samo za PRIKAZ predloga na praznoj formi (NoviNalog) — stvarni
// broj koji se upisuje generiše Kreiraj iznova, unutar svoje transakcije.
func (r *ServisRepo) SledeciBroj(ctx context.Context) (string, error) {
	return sledeciBrojServisa(ctx, r.db)
}

func sledeciBrojServisa(ctx context.Context, q interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}) (string, error) {
	sada := time.Now()
	// prefiks "SN-GGMM-" je dug 8 karaktera, pa brojač počinje od 9. karaktera
	prefiks := fmt.Sprintf("SN-%02d%02d-", sada.Year()%100, int(sada.Month()))
	uzorak := prefiks + "%"

	// COALESCE(MAX, 0)+1 → prvi nalog u mesecu dobija 001
	var sledeci int
	err := q.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(CAST(SUBSTR(broj_naloga, 9) AS INTEGER)), 0) + 1
		FROM servisni_nalozi
		WHERE broj_naloga LIKE ?`, uzorak).Scan(&sledeci)
	if err != nil {
		return "", fmt.Errorf("ntech: sledeciBrojServisa: %w", err)
	}

	return fmt.Sprintf("%s%03d", prefiks, sledeci), nil
}

// Lista vraća listu servisnih naloga sa imenom klijenta, opcionim filterima
func (r *ServisRepo) Lista(ctx context.Context, pretraga, status string) ([]model.ServisniNalogSaKlijentom, error) {
	upit := `
		SELECT
			sn.id, sn.klijent_id, sn.tehnicar_id, sn.broj_naloga, sn.uredjaj, sn.serijski_broj,
			sn.opis_kvara, sn.trazene_nadogradnje, sn.status, sn.cena_od, sn.cena_do, sn.cena_konacna,
			sn.avans, sn.napomena, sn.garancija_do, sn.garancija_dana, sn.datum_prijema, sn.datum_zavrsetka, sn.predvidjen_datum,
			sn.ostecenja, sn.pin_uredjaja, sn.pribor, sn.napomena_klijentu, sn.nalaz_dijagnostike, sn.uradjeno, sn.cena_dijagnostike, sn.popravka_odbijena, sn.javni_token, sn.komentar_klijenta, sn.odluka_klijenta, sn.datum_odluke, sn.nacin_placanja, sn.naplaceno, sn.stornirano, sn.razlog_storniranja,
			COALESCE(kp.naziv, '') AS klijent_naziv,
			(EXISTS(SELECT 1 FROM servis_radovi sr WHERE sr.nalog_id = sn.id AND sr.predlozeno = 1)
			 OR EXISTS(SELECT 1 FROM servisni_delovi sd WHERE sd.nalog_id = sn.id AND sd.predlozeno = 1)
			 OR EXISTS(SELECT 1 FROM servisni_potrazivani_delovi spd WHERE spd.nalog_id = sn.id AND spd.predlozeno = 1)) AS ima_predlog
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
		err := scanNalog(redovi.Scan, &n.ServisniNalog, &n.KlijentNaziv, &n.ImaPredlog)
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
			avans, napomena, garancija_do, garancija_dana, datum_prijema, datum_zavrsetka, predvidjen_datum,
			ostecenja, pin_uredjaja, pribor, napomena_klijentu, nalaz_dijagnostike, uradjeno, cena_dijagnostike, popravka_odbijena, javni_token, komentar_klijenta, odluka_klijenta, datum_odluke, nacin_placanja, naplaceno, stornirano, razlog_storniranja
		FROM servisni_nalozi WHERE id = ?`, id)

	var n model.ServisniNalog
	err := scanNalog(red.Scan, &n, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("ntech: ServisRepo.DohvatiID: %w", err)
	}

	return &n, nil
}

// Kreiraj upisuje novi servisni nalog u bazu i generiše javni token.
// Broj naloga se generiše OVDE, unutar transakcije upisa, i ne veruje se
// vrednosti iz forme (koja je samo predlog prikazan pri otvaranju prazne
// forme) — tako dupliran/ponovljen POST zahtev ne pravi dva zasebna naloga
// sa istim brojem/sadržajem.
func (r *ServisRepo) Kreiraj(ctx context.Context, n *model.ServisniNalog) (int64, error) {
	token, err := generisiJavniToken()
	if err != nil {
		return 0, fmt.Errorf("ntech: ServisRepo.Kreiraj: token: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("ntech: ServisRepo.Kreiraj: begin tx: %w", err)
	}
	defer tx.Rollback()

	// idempotency zaštita: isti obrazac kao ProdajaRepo.Kreiraj — ako pozivalac pošalje
	// ključ i nalog sa tim ključem već postoji, to je dupliran POST — vraćamo postojeći ID.
	if n.IdempotencyKey != "" {
		var postojeciID int64
		err := tx.QueryRowContext(ctx,
			"SELECT id FROM servisni_nalozi WHERE idempotency_key = ?", n.IdempotencyKey,
		).Scan(&postojeciID)
		if err == nil {
			return postojeciID, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return 0, fmt.Errorf("ntech: ServisRepo.Kreiraj: provera idempotency key: %w", err)
		}
	}

	brojNaloga, err := sledeciBrojServisa(ctx, tx)
	if err != nil {
		return 0, fmt.Errorf("ntech: ServisRepo.Kreiraj: broj naloga: %w", err)
	}
	n.BrojNaloga = brojNaloga

	rezultat, err := tx.ExecContext(ctx, `
		INSERT INTO servisni_nalozi
			(klijent_id, tehnicar_id, broj_naloga, uredjaj, serijski_broj, opis_kvara, trazene_nadogradnje,
			 status, cena_od, cena_do, cena_konacna, avans, napomena, garancija_do, garancija_dana, datum_zavrsetka, predvidjen_datum,
			 ostecenja, pin_uredjaja, pribor, datum_prijema, javni_token, idempotency_key)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		nullInt64(n.KlijentID), nullInt64(n.TehnicarID), n.BrojNaloga, n.Uredjaj,
		nullString(n.SerijskiBroj), n.OpisKvara, n.TrazeneNadogradnje, n.Status,
		nullFloat64(n.CenaOd), nullFloat64(n.CenaDo), nullFloat64(n.CenaKonacna),
		nullFloat64(n.Avans), nullString(n.Napomena),
		nullTime(n.GarancijaDo), nullInt(n.GarancijaDana), nullTime(n.DatumZavrsetka), nullTime(n.PredvidjenDatum),
		nullString(n.Ostecenja), nullString(n.PinUredjaja), nullString(n.Pribor),
		n.DatumPrijema, token, nullString(n.IdempotencyKey),
	)
	if err != nil {
		return 0, fmt.Errorf("ntech: ServisRepo.Kreiraj: %w", err)
	}

	id, err := rezultat.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("ntech: ServisRepo.Kreiraj: last insert id: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("ntech: ServisRepo.Kreiraj: commit: %w", err)
	}

	return id, nil
}

// DohvatiJavniToken vraća servisni nalog po javnom tokenu — bez autentifikacije
func (r *ServisRepo) DohvatiJavniToken(ctx context.Context, token string) (*model.ServisniNalog, error) {
	red := r.db.QueryRowContext(ctx, `
		SELECT
			id, klijent_id, tehnicar_id, broj_naloga, uredjaj, serijski_broj,
			opis_kvara, trazene_nadogradnje, status, cena_od, cena_do, cena_konacna,
			avans, napomena, garancija_do, garancija_dana, datum_prijema, datum_zavrsetka, predvidjen_datum,
			ostecenja, pin_uredjaja, pribor, napomena_klijentu, nalaz_dijagnostike, uradjeno, cena_dijagnostike, popravka_odbijena, javni_token, komentar_klijenta, odluka_klijenta, datum_odluke, nacin_placanja, naplaceno, stornirano, razlog_storniranja
		FROM servisni_nalozi WHERE javni_token = ?`, token)

	var n model.ServisniNalog
	if err := scanNalog(red.Scan, &n, nil, nil); err != nil {
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
			avans = ?, napomena = ?, garancija_do = ?, garancija_dana = ?, datum_zavrsetka = ?, predvidjen_datum = ?,
			ostecenja = ?, pin_uredjaja = ?, pribor = ?
		WHERE id = ?`,
		nullInt64(n.KlijentID), nullInt64(n.TehnicarID), n.Uredjaj, nullString(n.SerijskiBroj), n.OpisKvara, n.TrazeneNadogradnje,
		n.Status, nullFloat64(n.CenaOd), nullFloat64(n.CenaDo), nullFloat64(n.CenaKonacna),
		nullFloat64(n.Avans), nullString(n.Napomena), nullTime(n.GarancijaDo), nullInt(n.GarancijaDana), nullTime(n.DatumZavrsetka), nullTime(n.PredvidjenDatum),
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
	// svaka „normalna" promena statusa poništava oznaku odbijene popravke
	// (npr. serviser se predomisli i vrati nalog iz „Završeno" u dijagnostiku/popravku)
	var upit string
	if status == model.StatusZavrseno || status == model.StatusPreuzeto {
		upit = `UPDATE servisni_nalozi SET status = ?, popravka_odbijena = 0,
			datum_zavrsetka = COALESCE(datum_zavrsetka, date('now', 'localtime'))
			WHERE id = ?`
	} else {
		upit = `UPDATE servisni_nalozi SET status = ?, popravka_odbijena = 0 WHERE id = ?`
	}
	_, err := r.db.ExecContext(ctx, upit, status, id)
	if err != nil {
		return fmt.Errorf("ntech: ServisRepo.AzurirajStatus: %w", err)
	}
	return nil
}

// OdbijPopravku označava da je klijent odbio popravku posle dijagnostike:
// nalog prelazi u „Završeno", upisuje se oznaka i cena dijagnostike (taksa za pregled).
func (r *ServisRepo) OdbijPopravku(ctx context.Context, id int64, cena float64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE servisni_nalozi
		SET status = ?, popravka_odbijena = 1, cena_dijagnostike = ?,
			datum_zavrsetka = COALESCE(datum_zavrsetka, date('now', 'localtime'))
		WHERE id = ?`, model.StatusZavrseno, cena, id)
	if err != nil {
		return fmt.Errorf("ntech: ServisRepo.OdbijPopravku: %w", err)
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

// AzurirajGarancijaDana postavlja trajanje garancije u danima (od završetka radova).
// dana == nil ili 0 → bez garancije.
func (r *ServisRepo) AzurirajGarancijaDana(ctx context.Context, id int64, dana *int) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE servisni_nalozi SET garancija_dana = ? WHERE id = ?",
		nullInt(dana), id,
	)
	if err != nil {
		return fmt.Errorf("ntech: ServisRepo.AzurirajGarancijaDana: %w", err)
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

// AzurirajTehnicar postavlja ili uklanja dodeljenog servisera na nalogu.
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

// AzurirajNalazDijagnostike postavlja tekst nalaza dijagnostike (dijagnoza kvara i predlog popravke) na nalogu
func (r *ServisRepo) AzurirajNalazDijagnostike(ctx context.Context, id int64, tekst string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE servisni_nalozi SET nalaz_dijagnostike = ? WHERE id = ?", tekst, id,
	)
	if err != nil {
		return fmt.Errorf("ntech: ServisRepo.AzurirajNalazDijagnostike: %w", err)
	}
	return nil
}

// AzurirajUradjeno čuva tekst „šta je urađeno" na servisnom nalogu
func (r *ServisRepo) AzurirajUradjeno(ctx context.Context, id int64, tekst string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE servisni_nalozi SET uradjeno = ? WHERE id = ?", tekst, id,
	)
	if err != nil {
		return fmt.Errorf("ntech: ServisRepo.AzurirajUradjeno: %w", err)
	}
	return nil
}

// AzurirajCenaDijagnostike postavlja cenu dijagnostike (taksa kad klijent ne prihvati popravku)
func (r *ServisRepo) AzurirajCenaDijagnostike(ctx context.Context, id int64, cena float64) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE servisni_nalozi SET cena_dijagnostike = ? WHERE id = ?", cena, id,
	)
	if err != nil {
		return fmt.Errorf("ntech: ServisRepo.AzurirajCenaDijagnostike: %w", err)
	}
	return nil
}

func (r *ServisRepo) AzurirajCenuKonacnu(ctx context.Context, id int64, cena float64) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE servisni_nalozi SET cena_konacna = ? WHERE id = ?", cena, id,
	)
	if err != nil {
		return fmt.Errorf("ntech: ServisRepo.AzurirajCenuKonacnu: %w", err)
	}
	return nil
}

// AzurirajKomentarKlijenta čuva poruku klijenta uz prihvatanje/odbijanje predloga
func (r *ServisRepo) AzurirajKomentarKlijenta(ctx context.Context, id int64, tekst string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE servisni_nalozi SET komentar_klijenta = ? WHERE id = ?", tekst, id,
	)
	if err != nil {
		return fmt.Errorf("ntech: ServisRepo.AzurirajKomentarKlijenta: %w", err)
	}
	return nil
}

// ObrisiOdlukuKlijenta poništava prethodnu odluku klijenta (npr. kad se doda novi predlog).
func (r *ServisRepo) ObrisiOdlukuKlijenta(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE servisni_nalozi SET odluka_klijenta = NULL, komentar_klijenta = NULL, datum_odluke = NULL WHERE id = ?", id,
	)
	if err != nil {
		return fmt.Errorf("ntech: ServisRepo.ObrisiOdlukuKlijenta: %w", err)
	}
	return nil
}

// SacuvajOdlukuKlijenta beleži odluku klijenta (prihvaceno/odbijeno) i poruku.
// Ako je prihvaćeno, poziva PrihvatiPredlozene za delove i radove.
func (r *ServisRepo) SacuvajOdlukuKlijenta(ctx context.Context, id int64, odluka string, odgovor string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE servisni_nalozi SET odluka_klijenta = ?, komentar_klijenta = ?, datum_odluke = CURRENT_TIMESTAMP WHERE id = ?",
		odluka, odgovor, id,
	)
	if err != nil {
		return fmt.Errorf("ntech: ServisRepo.SacuvajOdlukuKlijenta: %w", err)
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
	if err := vratiStavkeNaStanje(ctx, tx, "servisni_delovi", id, korisnikID, "brisanje servisnog naloga"); err != nil {
		return fmt.Errorf("ntech: ServisRepo.Obrisi: %w", err)
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

// Storno stornira servisni nalog: vraća ugrađene delove na stanje (isto kao
// Obrisi) ali NALOG SE NE BRIŠE — samo se markira stornirano=1, po istom principu
// kao Prodaja. Finansijski zapisi (KIR/KPO/fiskalni) se ne diraju ovde — to radi
// handler posle uspešnog storna, dograđivanjem storno stavki na osnovu izvor_id.
func (r *ServisRepo) Storno(ctx context.Context, id int64, razlog string, korisnikID *int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ntech: ServisRepo.Storno: begin tx: %w", err)
	}
	defer tx.Rollback()

	var vecStornirano bool
	if err := tx.QueryRowContext(ctx, "SELECT stornirano FROM servisni_nalozi WHERE id = ?", id).Scan(&vecStornirano); err != nil {
		return fmt.Errorf("ntech: ServisRepo.Storno: provera: %w", err)
	}
	if vecStornirano {
		return fmt.Errorf("ntech: ServisRepo.Storno: nalog je već storniran")
	}

	if err := vratiStavkeNaStanje(ctx, tx, "servisni_delovi", id, korisnikID, "storno servisnog naloga"); err != nil {
		return fmt.Errorf("ntech: ServisRepo.Storno: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		"UPDATE servisni_nalozi SET stornirano = 1, razlog_storniranja = ? WHERE id = ?",
		nullString(razlog), id,
	); err != nil {
		return fmt.Errorf("ntech: ServisRepo.Storno: update: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("ntech: ServisRepo.Storno: commit: %w", err)
	}
	return nil
}

// SacuvajNaplatu beleži način plaćanja i naplaćeni iznos pri preuzimanju uređaja.
func (r *ServisRepo) SacuvajNaplatu(ctx context.Context, id int64, nacinPlacanja string, naplaceno float64) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE servisni_nalozi SET nacin_placanja = ?, naplaceno = ? WHERE id = ?",
		nacinPlacanja, naplaceno, id,
	)
	if err != nil {
		return fmt.Errorf("ntech: ServisRepo.SacuvajNaplatu: %w", err)
	}
	return nil
}

// scanNalog čita redove iz upita u ServisniNalog struct —
// klijentNaziv je opcioni pokazivač, nil kada se čita bez JOIN-a
func scanNalog(scan func(...any) error, n *model.ServisniNalog, klijentNaziv *string, imaPredlog *bool) error {
	var klijentID, tehnicarID sql.NullInt64
	var serijskiBroj, napomena, ostecenja, pinUredjaja, pribor, napomenaKlijentu, nalazDijagnostike, uradjeno, javniToken, komentarKlijenta, odlukaKlijenta sql.NullString
	var cenaOd, cenaDo, cenaKonacna, avans sql.NullFloat64
	var garancijaDo, datumZavrsetka, predvidjenDatum, datumOdluke sql.NullTime
	var garancijaDana sql.NullInt64
	var popravkaOdbijena sql.NullInt64

	var nacinPlacanja sql.NullString
	var naplaceno sql.NullFloat64
	var stornirano sql.NullInt64
	var razlogStorniranja sql.NullString

	args := []any{
		&n.ID, &klijentID, &tehnicarID, &n.BrojNaloga, &n.Uredjaj, &serijskiBroj,
		&n.OpisKvara, &n.TrazeneNadogradnje, &n.Status, &cenaOd, &cenaDo, &cenaKonacna,
		&avans, &napomena, &garancijaDo, &garancijaDana, &n.DatumPrijema, &datumZavrsetka, &predvidjenDatum,
		&ostecenja, &pinUredjaja, &pribor, &napomenaKlijentu, &nalazDijagnostike, &uradjeno, &n.CenaDijagnostike, &popravkaOdbijena, &javniToken, &komentarKlijenta, &odlukaKlijenta, &datumOdluke,
		&nacinPlacanja, &naplaceno, &stornirano, &razlogStorniranja,
	}

	if klijentNaziv != nil {
		args = append(args, klijentNaziv)
	}
	if imaPredlog != nil {
		args = append(args, imaPredlog)
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
	n.NalazDijagnostike = nalazDijagnostike.String
	n.Uradjeno = uradjeno.String
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
	if garancijaDana.Valid {
		v := int(garancijaDana.Int64)
		n.GarancijaDana = &v
	}
	n.PopravkaOdbijena = popravkaOdbijena.Int64 != 0
	n.JavniToken = javniToken.String
	n.KomentarKlijenta = komentarKlijenta.String
	n.OdlukaKlijenta = odlukaKlijenta.String
	if datumOdluke.Valid {
		v := datumOdluke.Time
		n.DatumOdluke = &v
	}
	n.NacinPlacanja = nacinPlacanja.String
	if naplaceno.Valid {
		n.Naplaceno = naplaceno.Float64
	}
	n.Stornirano = stornirano.Int64 != 0
	n.RazlogStorniranja = razlogStorniranja.String

	return nil
}
