package sqlite

import "database/sql"

// nullString pretvara prazan Go string u sql.NullString sa NULL vrednošću —
// koristi se pri unosu i izmeni kada polje u bazi sme biti NULL
func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

// nullInt64 pretvara *int64 pokazivač u sql.NullInt64 —
// koristi se za opciona FK polja koja smeju biti NULL u bazi
func nullInt64(v *int64) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *v, Valid: true}
}
