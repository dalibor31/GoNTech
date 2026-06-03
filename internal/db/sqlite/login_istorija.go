package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"ntech/internal/model"
)

// LoginIstorijsaRepo implementira db.LoginIstorijsaRepository nad SQLite
type LoginIstorijsaRepo struct {
	db *sql.DB
}

// NoviLoginIstorijsaRepo kreira novi repozitorijum za evidenciju prijava
func NoviLoginIstorijsaRepo(db *sql.DB) *LoginIstorijsaRepo {
	return &LoginIstorijsaRepo{db: db}
}

// Zabeleži upisuje pokušaj prijave u evidenciju
func (r *LoginIstorijsaRepo) Zabeleži(ctx context.Context, korisnikID *int64, ip, userAgent, razlog string, uspeh bool) error {
	u := 0
	if uspeh {
		u = 1
	}
	var kid sql.NullInt64
	if korisnikID != nil {
		kid = sql.NullInt64{Valid: true, Int64: *korisnikID}
	}
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO login_istorija (korisnik_id, ip, user_agent, uspeh, razlog) VALUES (?, ?, ?, ?, ?)",
		kid, ip, userAgent, u, razlog,
	)
	if err != nil {
		return fmt.Errorf("ntech: LoginIstorijsaRepo.Zabeleži: %w", err)
	}
	return nil
}

// ListaZaKorisnika vraća poslednjih N zapisa za datog korisnika
func (r *LoginIstorijsaRepo) ListaZaKorisnika(ctx context.Context, korisnikID int64, limit int) ([]*model.LoginPokusaj, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, korisnik_id, ip, user_agent, uspeh, razlog, vreme
		 FROM login_istorija
		 WHERE korisnik_id = ?
		 ORDER BY vreme DESC
		 LIMIT ?`,
		korisnikID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("ntech: LoginIstorijsaRepo.ListaZaKorisnika: %w", err)
	}
	defer rows.Close()

	var lista []*model.LoginPokusaj
	for rows.Next() {
		p := &model.LoginPokusaj{}
		var kid sql.NullInt64
		var u int
		var s string
		if err := rows.Scan(&p.ID, &kid, &p.IP, &p.UserAgent, &u, &p.Razlog, &s); err != nil {
			return nil, fmt.Errorf("ntech: LoginIstorijsaRepo.ListaZaKorisnika: scan: %w", err)
		}
		if kid.Valid {
			p.KorisnikID = &kid.Int64
		}
		p.Uspeh = u == 1
		p.Vreme, _ = time.ParseInLocation("2006-01-02 15:04:05", s, time.UTC)
		lista = append(lista, p)
	}
	return lista, rows.Err()
}
