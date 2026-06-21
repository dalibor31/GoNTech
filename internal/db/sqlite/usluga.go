package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"ntech/internal/db"
	"ntech/internal/model"
)

// UslugaRepo je SQLite implementacija db.UslugaRepository
type UslugaRepo struct {
	db *sql.DB
}

// NoviUslugaRepo kreira repozitorijum usluga nad datom bazom
func NoviUslugaRepo(baza *sql.DB) *UslugaRepo {
	return &UslugaRepo{db: baza}
}

// skenirajUslugu čita jedan red u model.Usluga
func skenirajUslugu(s interface {
	Scan(dest ...any) error
}) (model.Usluga, error) {
	var u model.Usluga
	var sifra sql.NullString
	var arhiviran int
	if err := s.Scan(&u.ID, &sifra, &u.Naziv, &u.Kategorija, &u.JedinicaMere, &u.Cena, &u.PdvStopa, &u.Opis, &arhiviran, &u.DatumUnosa); err != nil {
		return u, err
	}
	u.Sifra = sifra.String
	u.Arhiviran = arhiviran == 1
	return u, nil
}

const uslugaKolone = "id, sifra, naziv, kategorija, jedinica_mere, cena, pdv_stopa, opis, arhiviran, datum_unosa"

// Lista vraća usluge prema filteru (pretraga po nazivu/šifri/kategoriji, aktivne/arhivirane)
func (r *UslugaRepo) Lista(ctx context.Context, filter db.UslugaFilter) ([]model.Usluga, error) {
	upit := "SELECT " + uslugaKolone + " FROM usluge WHERE 1=1"
	var args []any

	if filter.Pretraga != "" {
		upit += " AND (naziv LIKE ? OR sifra LIKE ? OR kategorija LIKE ?)"
		t := "%" + filter.Pretraga + "%"
		args = append(args, t, t, t)
	}
	if filter.Arhivirani {
		upit += " AND arhiviran = 1"
	} else {
		upit += " AND arhiviran = 0"
	}
	upit += " ORDER BY naziv"
	if filter.Limit > 0 {
		upit += " LIMIT ? OFFSET ?"
		args = append(args, filter.Limit, filter.Offset)
	}

	redovi, err := r.db.QueryContext(ctx, upit, args...)
	if err != nil {
		return nil, fmt.Errorf("ntech: UslugaRepo.Lista: %w", err)
	}
	defer redovi.Close()

	var usluge []model.Usluga
	for redovi.Next() {
		u, err := skenirajUslugu(redovi)
		if err != nil {
			return nil, fmt.Errorf("ntech: UslugaRepo.Lista: %w", err)
		}
		usluge = append(usluge, u)
	}
	return usluge, redovi.Err()
}

// PrebrojiPoFilteru vraća broj usluga koje zadovoljavaju filter (za paginaciju)
func (r *UslugaRepo) PrebrojiPoFilteru(ctx context.Context, filter db.UslugaFilter) (int, error) {
	upit := "SELECT COUNT(*) FROM usluge WHERE 1=1"
	var args []any
	if filter.Pretraga != "" {
		upit += " AND (naziv LIKE ? OR sifra LIKE ? OR kategorija LIKE ?)"
		t := "%" + filter.Pretraga + "%"
		args = append(args, t, t, t)
	}
	if filter.Arhivirani {
		upit += " AND arhiviran = 1"
	} else {
		upit += " AND arhiviran = 0"
	}
	var n int
	if err := r.db.QueryRowContext(ctx, upit, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("ntech: UslugaRepo.PrebrojiPoFilteru: %w", err)
	}
	return n, nil
}

// DohvatiID vraća jednu uslugu po ID-u
func (r *UslugaRepo) DohvatiID(ctx context.Context, id int64) (*model.Usluga, error) {
	red := r.db.QueryRowContext(ctx, "SELECT "+uslugaKolone+" FROM usluge WHERE id = ?", id)
	u, err := skenirajUslugu(red)
	if err != nil {
		return nil, fmt.Errorf("ntech: UslugaRepo.DohvatiID: %w", err)
	}
	return &u, nil
}

// Kreiraj upisuje novu uslugu i vraća njen ID
func (r *UslugaRepo) Kreiraj(ctx context.Context, u *model.Usluga) (int64, error) {
	var sifra any
	if u.Sifra != "" {
		sifra = u.Sifra
	}
	rez, err := r.db.ExecContext(ctx, `
		INSERT INTO usluge (sifra, naziv, kategorija, jedinica_mere, cena, pdv_stopa, opis)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		sifra, u.Naziv, u.Kategorija, u.JedinicaMere, u.Cena, u.PdvStopa, u.Opis,
	)
	if err != nil {
		return 0, fmt.Errorf("ntech: UslugaRepo.Kreiraj: %w", err)
	}
	id, err := rez.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("ntech: UslugaRepo.Kreiraj: last insert id: %w", err)
	}
	return id, nil
}

// Izmeni ažurira postojeću uslugu
func (r *UslugaRepo) Izmeni(ctx context.Context, u *model.Usluga) error {
	var sifra any
	if u.Sifra != "" {
		sifra = u.Sifra
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE usluge SET sifra = ?, naziv = ?, kategorija = ?, jedinica_mere = ?, cena = ?, pdv_stopa = ?, opis = ?
		WHERE id = ?`,
		sifra, u.Naziv, u.Kategorija, u.JedinicaMere, u.Cena, u.PdvStopa, u.Opis, u.ID,
	)
	if err != nil {
		return fmt.Errorf("ntech: UslugaRepo.Izmeni: %w", err)
	}
	return nil
}

// Obrisi trajno briše uslugu po ID-u
func (r *UslugaRepo) Obrisi(ctx context.Context, id int64) error {
	if _, err := r.db.ExecContext(ctx, "DELETE FROM usluge WHERE id = ?", id); err != nil {
		return fmt.Errorf("ntech: UslugaRepo.Obrisi: %w", err)
	}
	return nil
}

// Kategorije vraća distinct neprazne kategorije usluga (za predlog pri unosu)
func (r *UslugaRepo) Kategorije(ctx context.Context) ([]string, error) {
	redovi, err := r.db.QueryContext(ctx, "SELECT DISTINCT kategorija FROM usluge WHERE kategorija != '' ORDER BY kategorija")
	if err != nil {
		return nil, fmt.Errorf("ntech: UslugaRepo.Kategorije: %w", err)
	}
	defer redovi.Close()
	var kat []string
	for redovi.Next() {
		var k string
		if err := redovi.Scan(&k); err != nil {
			return nil, fmt.Errorf("ntech: UslugaRepo.Kategorije: %w", err)
		}
		kat = append(kat, k)
	}
	return kat, redovi.Err()
}

// SledecaSifra vraća predlog sledeće auto-šifre usluge u formatu USL-001 … USL-999
func (r *UslugaRepo) SledecaSifra(ctx context.Context) (string, error) {
	var maks sql.NullInt64
	err := r.db.QueryRowContext(ctx,
		"SELECT MAX(CAST(SUBSTR(sifra, 5) AS INTEGER)) FROM usluge WHERE sifra LIKE 'USL-%'").Scan(&maks)
	if err != nil {
		return "USL-001", fmt.Errorf("ntech: UslugaRepo.SledecaSifra: %w", err)
	}
	return fmt.Sprintf("USL-%03d", maks.Int64+1), nil
}
