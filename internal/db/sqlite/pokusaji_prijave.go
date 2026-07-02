package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"ntech/internal/model"
)

// PokusajiPrijaveRepo implementira db.PokusajiPrijaveRepository nad SQLite
type PokusajiPrijaveRepo struct {
	db *sql.DB
}

// NoviPokusajiPrijaveRepo kreira novi repozitorijum za praćenje pokušaja prijave
func NoviPokusajiPrijaveRepo(db *sql.DB) *PokusajiPrijaveRepo {
	return &PokusajiPrijaveRepo{db: db}
}

// Zabeleži upisuje novi pokušaj prijave (uspešan ili neuspešan)
func (r *PokusajiPrijaveRepo) Zabeleži(ctx context.Context, ip, korisnickoIme string, uspeh bool) error {
	u := 0
	if uspeh {
		u = 1
	}
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO pokusaji_prijave (ip, korisnicko_ime, uspeh) VALUES (?, ?, ?)",
		ip, korisnickoIme, u,
	)
	if err != nil {
		return fmt.Errorf("ntech: PokusajiPrijaveRepo.Zabeleži: %w", err)
	}
	return nil
}

// BrojNeuspeha vraća broj neuspelih pokušaja za datu IP adresu od zadatog trenutka
func (r *PokusajiPrijaveRepo) BrojNeuspeha(ctx context.Context, ip string, od time.Time) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM pokusaji_prijave WHERE ip = ? AND uspeh = 0 AND vreme > ?",
		ip, od.UTC().Format("2006-01-02 15:04:05"),
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("ntech: PokusajiPrijaveRepo.BrojNeuspeha: %w", err)
	}
	return n, nil
}

// VremePoslednjeg vraća vreme poslednjeg neuspelog pokušaja za datu IP adresu od zadatog trenutka
func (r *PokusajiPrijaveRepo) VremePoslednjeg(ctx context.Context, ip string, od time.Time) (time.Time, bool, error) {
	var s sql.NullString
	err := r.db.QueryRowContext(ctx,
		"SELECT MAX(vreme) FROM pokusaji_prijave WHERE ip = ? AND uspeh = 0 AND vreme > ?",
		ip, od.UTC().Format("2006-01-02 15:04:05"),
	).Scan(&s)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("ntech: PokusajiPrijaveRepo.VremePoslednjeg: %w", err)
	}
	if !s.Valid || s.String == "" {
		return time.Time{}, false, nil
	}
	t, err := time.ParseInLocation("2006-01-02 15:04:05", s.String, time.UTC)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("ntech: PokusajiPrijaveRepo.VremePoslednjeg: parse: %w", err)
	}
	return t, true, nil
}

// ObrisiStare briše sve zapise starije od zadatog trenutka
func (r *PokusajiPrijaveRepo) ObrisiStare(ctx context.Context, pre time.Time) error {
	_, err := r.db.ExecContext(ctx,
		"DELETE FROM pokusaji_prijave WHERE vreme < ?",
		pre.UTC().Format("2006-01-02 15:04:05"),
	)
	if err != nil {
		return fmt.Errorf("ntech: PokusajiPrijaveRepo.ObrisiStare: %w", err)
	}
	return nil
}

// ListaBlokiranih vraća IP adrese sa najmanje prag neuspelih pokušaja prijave od
// zadatog trenutka, grupisano po IP adresi, poređano od najskorije zaključane.
func (r *PokusajiPrijaveRepo) ListaBlokiranih(ctx context.Context, od time.Time, prag int) ([]model.BlokiranaIP, error) {
	redovi, err := r.db.QueryContext(ctx,
		`SELECT ip, COUNT(*), MAX(vreme) FROM pokusaji_prijave
		 WHERE uspeh = 0 AND vreme > ?
		 GROUP BY ip HAVING COUNT(*) >= ?
		 ORDER BY MAX(vreme) DESC`,
		od.UTC().Format("2006-01-02 15:04:05"), prag,
	)
	if err != nil {
		return nil, fmt.Errorf("ntech: PokusajiPrijaveRepo.ListaBlokiranih: %w", err)
	}
	defer redovi.Close()

	var lista []model.BlokiranaIP
	for redovi.Next() {
		var b model.BlokiranaIP
		var poslednji string
		if err := redovi.Scan(&b.IP, &b.BrojNeuspeha, &poslednji); err != nil {
			return nil, fmt.Errorf("ntech: PokusajiPrijaveRepo.ListaBlokiranih: scan: %w", err)
		}
		t, err := time.ParseInLocation("2006-01-02 15:04:05", poslednji, time.UTC)
		if err != nil {
			return nil, fmt.Errorf("ntech: PokusajiPrijaveRepo.ListaBlokiranih: parse vremena: %w", err)
		}
		b.PoslednjiPokusaj = t
		lista = append(lista, b)
	}
	if err := redovi.Err(); err != nil {
		return nil, fmt.Errorf("ntech: PokusajiPrijaveRepo.ListaBlokiranih: %w", err)
	}
	return lista, nil
}

// Odblokiraj briše sve neuspele pokušaje prijave za datu IP adresu — brojač
// neuspeha odmah pada na nulu, pa IP prestaje da bude zaključana.
func (r *PokusajiPrijaveRepo) Odblokiraj(ctx context.Context, ip string) error {
	_, err := r.db.ExecContext(ctx,
		"DELETE FROM pokusaji_prijave WHERE ip = ? AND uspeh = 0",
		ip,
	)
	if err != nil {
		return fmt.Errorf("ntech: PokusajiPrijaveRepo.Odblokiraj: %w", err)
	}
	return nil
}
