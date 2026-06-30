package sqlite

import (
	"context"
	"database/sql"

	"ntech/internal/model"
)

// ServisniLogRepo je SQLite implementacija db.ServisniLogRepository
type ServisniLogRepo struct {
	db *sql.DB
}

// NoviServisniLogRepo kreira repozitorijum log-a servisnih naloga
func NoviServisniLogRepo(baza *sql.DB) *ServisniLogRepo {
	return &ServisniLogRepo{db: baza}
}

// Kreiraj upisuje novi događaj u log servisnog naloga.
func (r *ServisniLogRepo) Kreiraj(ctx context.Context, nalogID int64, dogadjaj string, korisnikID *int64) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO servisni_log (nalog_id, dogadjaj, korisnik_id) VALUES (?, ?, ?)`,
		nalogID, dogadjaj, korisnikID,
	)
	return err
}

// DohvatiZaNalog vraća sve log zapise za dati nalog, od najstarijeg ka najnovijem.
func (r *ServisniLogRepo) DohvatiZaNalog(ctx context.Context, nalogID int64) ([]model.ServisniLog, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, nalog_id, dogadjaj, COALESCE(napomena,''), korisnik_id, datum
		 FROM servisni_log WHERE nalog_id = ? ORDER BY datum ASC`,
		nalogID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lista []model.ServisniLog
	for rows.Next() {
		var l model.ServisniLog
		if err := rows.Scan(&l.ID, &l.NalogID, &l.Dogadjaj, &l.Napomena, &l.KorisnikID, &l.Datum); err != nil {
			return nil, err
		}
		lista = append(lista, l)
	}
	return lista, rows.Err()
}
