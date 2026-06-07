package middleware

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"ntech/internal/model"
)

// SetFlash čuva flash poruku u koloni flash aktivne sesije
func SetFlash(w http.ResponseWriter, r *http.Request, db *sql.DB, tip, poruka string) {
	kolacic, err := r.Cookie("ntech_sesija")
	if err != nil {
		return
	}
	data, err := json.Marshal(model.FlashPoruka{Tip: tip, Poruka: poruka})
	if err != nil {
		return
	}
	db.ExecContext(r.Context(),
		`UPDATE sesije SET flash = ? WHERE token = ?`,
		string(data), kolacic.Value)
}

// GetFlash čita i atomično briše flash poruku iz aktivne sesije
func GetFlash(r *http.Request, db *sql.DB) *model.FlashPoruka {
	kolacic, err := r.Cookie("ntech_sesija")
	if err != nil {
		return nil
	}

	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		return nil
	}
	defer tx.Rollback()

	var flashJSON sql.NullString
	if err := tx.QueryRowContext(r.Context(),
		`SELECT flash FROM sesije WHERE token = ?`, kolacic.Value).Scan(&flashJSON); err != nil {
		return nil
	}
	if !flashJSON.Valid || flashJSON.String == "" {
		return nil
	}

	if _, err := tx.ExecContext(r.Context(),
		`UPDATE sesije SET flash = NULL WHERE token = ?`, kolacic.Value); err != nil {
		return nil
	}

	if err := tx.Commit(); err != nil {
		return nil
	}

	var f model.FlashPoruka
	if err := json.Unmarshal([]byte(flashJSON.String), &f); err != nil {
		return nil
	}
	return &f
}
