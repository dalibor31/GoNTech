package sqlite

import (
	"database/sql"
	"time"
)

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

// nullFloat64 pretvara *float64 pokazivač u sql.NullFloat64 —
// koristi se za opciona numerička polja kao što su cene
func nullFloat64(v *float64) sql.NullFloat64 {
	if v == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *v, Valid: true}
}

// nullTime pretvara *time.Time pokazivač u sql.NullTime —
// koristi se za opciona datumska polja kao što je datum završetka
func nullTime(v *time.Time) sql.NullTime {
	if v == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *v, Valid: true}
}
