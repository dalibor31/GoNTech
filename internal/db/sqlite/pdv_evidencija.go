package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"ntech/internal/model"
)

// --- KIR: knjiga izdatih računa ---

// PdvKirRepo je SQLite implementacija PdvKirRepository interfejsa
type PdvKirRepo struct {
	db *sql.DB
}

// NoviPdvKirRepo kreira novi PdvKirRepo
func NoviPdvKirRepo(db *sql.DB) *PdvKirRepo {
	return &PdvKirRepo{db: db}
}

// Lista vraća zapise KIR-a u zadatom periodu (po datumu prometa); nulti datum znači bez granice
func (r *PdvKirRepo) Lista(ctx context.Context, od, do time.Time) ([]model.PdvKir, error) {
	upit := `
		SELECT id, datum_prometa, datum_knjizenja, broj_dokumenta,
		       kupac_naziv, COALESCE(kupac_pib, ''), COALESCE(kupac_mesto, ''),
		       osnovica_opsta, pdv_opsta, osnovica_posebna, pdv_posebna,
		       osloboden_sa_pravom, osloboden_bez_prava, ukupno,
		       COALESCE(napomena, ''), izvor, izvor_id, datum_unosa
		FROM pdv_kir WHERE 1=1`
	args := []any{}
	if !od.IsZero() {
		upit += " AND datum_prometa >= ?"
		args = append(args, od)
	}
	if !do.IsZero() {
		upit += " AND datum_prometa <= ?"
		args = append(args, do)
	}
	upit += " ORDER BY datum_prometa ASC, id ASC"

	redovi, err := r.db.QueryContext(ctx, upit, args...)
	if err != nil {
		return nil, fmt.Errorf("ntech: PdvKirRepo.Lista: %w", err)
	}
	defer redovi.Close()

	var rezultat []model.PdvKir
	for redovi.Next() {
		k, err := skenirajKir(redovi.Scan)
		if err != nil {
			return nil, fmt.Errorf("ntech: PdvKirRepo.Lista: scan: %w", err)
		}
		rezultat = append(rezultat, k)
	}
	if err := redovi.Err(); err != nil {
		return nil, fmt.Errorf("ntech: PdvKirRepo.Lista: %w", err)
	}
	return rezultat, nil
}

// DohvatiID vraća jedan zapis KIR-a po identifikatoru
func (r *PdvKirRepo) DohvatiID(ctx context.Context, id int64) (*model.PdvKir, error) {
	red := r.db.QueryRowContext(ctx, `
		SELECT id, datum_prometa, datum_knjizenja, broj_dokumenta,
		       kupac_naziv, COALESCE(kupac_pib, ''), COALESCE(kupac_mesto, ''),
		       osnovica_opsta, pdv_opsta, osnovica_posebna, pdv_posebna,
		       osloboden_sa_pravom, osloboden_bez_prava, ukupno,
		       COALESCE(napomena, ''), izvor, izvor_id, datum_unosa
		FROM pdv_kir WHERE id = ?`, id)
	k, err := skenirajKir(red.Scan)
	if err != nil {
		return nil, fmt.Errorf("ntech: PdvKirRepo.DohvatiID: %w", err)
	}
	return &k, nil
}

// skenirajKir čita jedan red KIR-a; izvor_id je nullable pa ide preko sql.NullInt64
func skenirajKir(scan func(...any) error) (model.PdvKir, error) {
	var k model.PdvKir
	var izvorID sql.NullInt64
	if err := scan(
		&k.ID, &k.DatumPrometa, &k.DatumKnjizenja, &k.BrojDokumenta,
		&k.KupacNaziv, &k.KupacPib, &k.KupacMesto,
		&k.OsnovicaOpsta, &k.PdvOpsta, &k.OsnovicaPosebna, &k.PdvPosebna,
		&k.OslobodenSaPravom, &k.OslobodenBezPrava, &k.Ukupno,
		&k.Napomena, &k.Izvor, &izvorID, &k.DatumUnosa,
	); err != nil {
		return model.PdvKir{}, err
	}
	if izvorID.Valid {
		k.IzvorID = &izvorID.Int64
	}
	return k, nil
}

// Kreiraj dodaje novi zapis u KIR i vraća njegov id
func (r *PdvKirRepo) Kreiraj(ctx context.Context, k *model.PdvKir) (int64, error) {
	rez, err := r.db.ExecContext(ctx, `
		INSERT INTO pdv_kir (
			datum_prometa, datum_knjizenja, broj_dokumenta,
			kupac_naziv, kupac_pib, kupac_mesto,
			osnovica_opsta, pdv_opsta, osnovica_posebna, pdv_posebna,
			osloboden_sa_pravom, osloboden_bez_prava, ukupno, napomena, izvor, izvor_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		k.DatumPrometa, k.DatumKnjizenja, k.BrojDokumenta,
		k.KupacNaziv, k.KupacPib, k.KupacMesto,
		k.OsnovicaOpsta, k.PdvOpsta, k.OsnovicaPosebna, k.PdvPosebna,
		k.OslobodenSaPravom, k.OslobodenBezPrava, k.Ukupno, k.Napomena,
		izvorIliRucno(k.Izvor), izvorIDArg(k.IzvorID))
	if err != nil {
		return 0, fmt.Errorf("ntech: PdvKirRepo.Kreiraj: %w", err)
	}
	id, err := rez.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("ntech: PdvKirRepo.Kreiraj: id: %w", err)
	}
	return id, nil
}

// Obrisi briše zapis KIR-a
func (r *PdvKirRepo) Obrisi(ctx context.Context, id int64) error {
	if _, err := r.db.ExecContext(ctx, "DELETE FROM pdv_kir WHERE id = ?", id); err != nil {
		return fmt.Errorf("ntech: PdvKirRepo.Obrisi: %w", err)
	}
	return nil
}

// ObrisiPoIzvoru briše KIR zapise vezane za dati izvor (npr. pri stornu/brisanju prodaje)
func (r *PdvKirRepo) ObrisiPoIzvoru(ctx context.Context, izvor string, izvorID int64) error {
	if _, err := r.db.ExecContext(ctx, "DELETE FROM pdv_kir WHERE izvor = ? AND izvor_id = ?", izvor, izvorID); err != nil {
		return fmt.Errorf("ntech: PdvKirRepo.ObrisiPoIzvoru: %w", err)
	}
	return nil
}

// izvorIliRucno vraća izvor ili podrazumevano „rucno" (izvor kolona je NOT NULL)
func izvorIliRucno(izvor string) string {
	if izvor == "" {
		return "rucno"
	}
	return izvor
}

// izvorIDArg pretvara *int64 u argument za upit (nil → NULL)
func izvorIDArg(id *int64) any {
	if id == nil {
		return nil
	}
	return *id
}

// --- KPR: knjiga primljenih računa ---

// PdvKprRepo je SQLite implementacija PdvKprRepository interfejsa
type PdvKprRepo struct {
	db *sql.DB
}

// NoviPdvKprRepo kreira novi PdvKprRepo
func NoviPdvKprRepo(db *sql.DB) *PdvKprRepo {
	return &PdvKprRepo{db: db}
}

// Lista vraća zapise KPR-a u zadatom periodu (po datumu prometa); nulti datum znači bez granice
func (r *PdvKprRepo) Lista(ctx context.Context, od, do time.Time) ([]model.PdvKpr, error) {
	upit := `
		SELECT id, datum_prometa, datum_knjizenja, datum_placanja, broj_dokumenta,
		       dobavljac_naziv, COALESCE(dobavljac_pib, ''), COALESCE(dobavljac_mesto, ''),
		       osnovica_opsta, pdv_opsta, osnovica_posebna, pdv_posebna,
		       pdv_bez_odbitka, osloboden_nabavka, ukupno,
		       COALESCE(napomena, ''), izvor, izvor_id, uvoz, datum_unosa
		FROM pdv_kpr WHERE 1=1`
	args := []any{}
	if !od.IsZero() {
		upit += " AND datum_prometa >= ?"
		args = append(args, od)
	}
	if !do.IsZero() {
		upit += " AND datum_prometa <= ?"
		args = append(args, do)
	}
	upit += " ORDER BY datum_prometa ASC, id ASC"

	redovi, err := r.db.QueryContext(ctx, upit, args...)
	if err != nil {
		return nil, fmt.Errorf("ntech: PdvKprRepo.Lista: %w", err)
	}
	defer redovi.Close()

	var rezultat []model.PdvKpr
	for redovi.Next() {
		k, err := skenirajKpr(redovi.Scan)
		if err != nil {
			return nil, fmt.Errorf("ntech: PdvKprRepo.Lista: scan: %w", err)
		}
		rezultat = append(rezultat, k)
	}
	if err := redovi.Err(); err != nil {
		return nil, fmt.Errorf("ntech: PdvKprRepo.Lista: %w", err)
	}
	return rezultat, nil
}

// DohvatiID vraća jedan zapis KPR-a po identifikatoru
func (r *PdvKprRepo) DohvatiID(ctx context.Context, id int64) (*model.PdvKpr, error) {
	red := r.db.QueryRowContext(ctx, `
		SELECT id, datum_prometa, datum_knjizenja, datum_placanja, broj_dokumenta,
		       dobavljac_naziv, COALESCE(dobavljac_pib, ''), COALESCE(dobavljac_mesto, ''),
		       osnovica_opsta, pdv_opsta, osnovica_posebna, pdv_posebna,
		       pdv_bez_odbitka, osloboden_nabavka, ukupno,
		       COALESCE(napomena, ''), izvor, izvor_id, uvoz, datum_unosa
		FROM pdv_kpr WHERE id = ?`, id)
	k, err := skenirajKpr(red.Scan)
	if err != nil {
		return nil, fmt.Errorf("ntech: PdvKprRepo.DohvatiID: %w", err)
	}
	return &k, nil
}

// skenirajKpr čita jedan red KPR-a; datum_placanja je nullable pa ide preko sql.NullTime
func skenirajKpr(scan func(...any) error) (model.PdvKpr, error) {
	var k model.PdvKpr
	var datumPlacanja sql.NullTime
	var izvorID sql.NullInt64
	var uvoz int64
	if err := scan(
		&k.ID, &k.DatumPrometa, &k.DatumKnjizenja, &datumPlacanja, &k.BrojDokumenta,
		&k.DobavljacNaziv, &k.DobavljacPib, &k.DobavljacMesto,
		&k.OsnovicaOpsta, &k.PdvOpsta, &k.OsnovicaPosebna, &k.PdvPosebna,
		&k.PdvBezOdbitka, &k.OslobodenNabavka, &k.Ukupno,
		&k.Napomena, &k.Izvor, &izvorID, &uvoz, &k.DatumUnosa,
	); err != nil {
		return model.PdvKpr{}, err
	}
	if datumPlacanja.Valid {
		k.DatumPlacanja = &datumPlacanja.Time
	}
	if izvorID.Valid {
		k.IzvorID = &izvorID.Int64
	}
	k.Uvoz = uvoz != 0
	return k, nil
}

// Kreiraj dodaje novi zapis u KPR i vraća njegov id
func (r *PdvKprRepo) Kreiraj(ctx context.Context, k *model.PdvKpr) (int64, error) {
	var datumPlacanja any
	if k.DatumPlacanja != nil {
		datumPlacanja = *k.DatumPlacanja
	}
	uvoz := 0
	if k.Uvoz {
		uvoz = 1
	}
	rez, err := r.db.ExecContext(ctx, `
		INSERT INTO pdv_kpr (
			datum_prometa, datum_knjizenja, datum_placanja, broj_dokumenta,
			dobavljac_naziv, dobavljac_pib, dobavljac_mesto,
			osnovica_opsta, pdv_opsta, osnovica_posebna, pdv_posebna,
			pdv_bez_odbitka, osloboden_nabavka, ukupno, napomena, izvor, izvor_id, uvoz
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		k.DatumPrometa, k.DatumKnjizenja, datumPlacanja, k.BrojDokumenta,
		k.DobavljacNaziv, k.DobavljacPib, k.DobavljacMesto,
		k.OsnovicaOpsta, k.PdvOpsta, k.OsnovicaPosebna, k.PdvPosebna,
		k.PdvBezOdbitka, k.OslobodenNabavka, k.Ukupno, k.Napomena,
		izvorIliRucno(k.Izvor), izvorIDArg(k.IzvorID), uvoz)
	if err != nil {
		return 0, fmt.Errorf("ntech: PdvKprRepo.Kreiraj: %w", err)
	}
	id, err := rez.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("ntech: PdvKprRepo.Kreiraj: id: %w", err)
	}
	return id, nil
}

// Obrisi briše zapis KPR-a
func (r *PdvKprRepo) Obrisi(ctx context.Context, id int64) error {
	if _, err := r.db.ExecContext(ctx, "DELETE FROM pdv_kpr WHERE id = ?", id); err != nil {
		return fmt.Errorf("ntech: PdvKprRepo.Obrisi: %w", err)
	}
	return nil
}

// ObrisiPoIzvoru briše KPR zapise vezane za dati izvor (npr. pri brisanju nabavke)
func (r *PdvKprRepo) ObrisiPoIzvoru(ctx context.Context, izvor string, izvorID int64) error {
	if _, err := r.db.ExecContext(ctx, "DELETE FROM pdv_kpr WHERE izvor = ? AND izvor_id = ?", izvor, izvorID); err != nil {
		return fmt.Errorf("ntech: PdvKprRepo.ObrisiPoIzvoru: %w", err)
	}
	return nil
}

// PostojiZaIzvor vraća true ako postoji bar jedan KPR zapis za dati izvor i izvorID
func (r *PdvKprRepo) PostojiZaIzvor(ctx context.Context, izvor string, izvorID int64) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM pdv_kpr WHERE izvor = ? AND izvor_id = ?", izvor, izvorID,
	).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("ntech: PdvKprRepo.PostojiZaIzvor: %w", err)
	}
	return n > 0, nil
}
