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

// nullInt pretvara *int pokazivač u sql.NullInt64 —
// koristi se za opciona celobrojna polja (npr. trajanje garancije u danima)
func nullInt(v *int) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*v), Valid: true}
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

// nullDateString upisuje *time.Time kao "YYYY-MM-DD" string ili NULL —
// koristiti za DATE kolone gde sql.NullTime nije kompatibilan s driver-om
func nullDateString(v *time.Time) sql.NullString {
	if v == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: v.Format("2006-01-02"), Valid: true}
}

// parseDatumUnosa parsira "YYYY-MM-DD HH:MM:SS" iz kolone deklarisane kao TEXT
// (npr. usluge/troskovi.datum_unosa, `DEFAULT (datetime('now'))`). Driver
// vraća string umesto time.Time za TEXT kolone (za razliku od DATETIME kolona
// koje driver sam parsira), pa Scan ne može direktno u *time.Time.
func parseDatumUnosa(s string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02 15:04:05", s, time.UTC)
}
