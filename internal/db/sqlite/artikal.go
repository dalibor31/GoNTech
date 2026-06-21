package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"ntech/internal/db"
	"ntech/internal/model"

	mosqlite "modernc.org/sqlite"
)

// ArtikalRepo je SQLite implementacija ArtikalRepository interfejsa
type ArtikalRepo struct {
	db *sql.DB
}

// NoviArtikalRepo kreira novi ArtikalRepo
func NoviArtikalRepo(db *sql.DB) *ArtikalRepo {
	return &ArtikalRepo{db: db}
}

// Lista vraća listu artikala sa opcionalnim filterima
func (r *ArtikalRepo) Lista(ctx context.Context, filter db.ArtikalFilter) ([]model.ArtikalSaKategorijom, error) {
	upit := `
		SELECT
			a.id, a.kategorija_id, a.sifra, a.barkod, a.naziv, a.opis, a.tip, a.jedinica_mere,
			a.kolicina, a.kolicina_min, a.lokacija,
			a.nabavna_cena, a.prodajna_cena, a.pdv_stopa, a.marza, a.napomena, a.datum_unosa, a.arhiviran,
			COALESCE(k.naziv, '') as kategorija_naziv, k.marza as kategorija_marza
		FROM artikli a
		LEFT JOIN kategorije k ON a.kategorija_id = k.id
		WHERE 1=1`

	args := []any{}

	if filter.Pretraga != "" {
		upit += " AND (a.naziv LIKE ? OR a.sifra LIKE ? OR a.barkod LIKE ? OR a.lokacija LIKE ? OR k.naziv LIKE ?)"
		t := "%" + filter.Pretraga + "%"
		args = append(args, t, t, t, t, t)
	}

	if filter.KategorijaID != nil {
		upit += " AND a.kategorija_id = ?"
		args = append(args, *filter.KategorijaID)
	}

	if filter.Tip != "" {
		upit += " AND a.tip = ?"
		args = append(args, filter.Tip)
	}

	if filter.SamoKriticni {
		upit += " AND (a.tip = 'proizvod' OR a.tip = '') AND a.kolicina <= a.kolicina_min"
	}

	// podrazumevano vraćamo samo aktivne; arhivirani se prikazuju samo na izričit zahtev
	if filter.Arhivirani {
		upit += " AND a.arhiviran = 1"
	} else {
		upit += " AND a.arhiviran = 0"
	}

	upit += " ORDER BY a.naziv ASC"

	if filter.Limit > 0 {
		upit += " LIMIT ?"
		args = append(args, filter.Limit)
		if filter.Offset > 0 {
			upit += " OFFSET ?"
			args = append(args, filter.Offset)
		}
	}

	redovi, err := r.db.QueryContext(ctx, upit, args...)
	if err != nil {
		return nil, fmt.Errorf("ntech: ArtikalRepo.Lista: %w", err)
	}
	defer redovi.Close()

	var rezultat []model.ArtikalSaKategorijom
	for redovi.Next() {
		var a model.ArtikalSaKategorijom
		var kategorijaID sql.NullInt64
		var sifra, barkod sql.NullString
		var marza, katMarza sql.NullFloat64
		var arhiviran int

		err := redovi.Scan(
			&a.ID, &kategorijaID, &sifra, &barkod, &a.Naziv, &a.Opis, &a.Tip, &a.JedinicaMere,
			&a.Kolicina, &a.KolicinMin, &a.Lokacija,
			&a.NabavnaCena, &a.ProdajnaCena, &a.PdvStopa, &marza, &a.Napomena, &a.DatumUnosa, &arhiviran,
			&a.KategorijaNaziv, &katMarza,
		)
		if err != nil {
			return nil, fmt.Errorf("ntech: ArtikalRepo.Lista: scan: %w", err)
		}
		a.Arhiviran = arhiviran == 1

		if kategorijaID.Valid {
			a.KategorijaID = &kategorijaID.Int64
		}
		if sifra.Valid {
			a.Sifra = sifra.String
		}
		if barkod.Valid {
			a.Barkod = barkod.String
		}
		if marza.Valid {
			a.Marza = &marza.Float64
		}
		if katMarza.Valid {
			a.KategorijaMarza = &katMarza.Float64
		}

		// kritična zaliha važi samo za proizvode (usluge/troškovi nemaju lager)
		a.KriticnaZaliha = a.PratiLager() && a.Kolicina <= a.KolicinMin

		rezultat = append(rezultat, a)
	}

	return rezultat, nil
}

// DohvatiID vraća jedan artikal po ID-u
func (r *ArtikalRepo) DohvatiID(ctx context.Context, id int64) (*model.Artikal, error) {
	var a model.Artikal
	var kategorijaID sql.NullInt64
	var sifra, barkod sql.NullString
	var marza sql.NullFloat64
	var arhiviran int

	err := r.db.QueryRowContext(ctx, `
		SELECT id, kategorija_id, sifra, barkod, naziv, opis, tip, jedinica_mere, kolicina, kolicina_min,
		       lokacija, nabavna_cena, prodajna_cena, pdv_stopa, marza, napomena, datum_unosa, arhiviran
		FROM artikli WHERE id = ?`, id).Scan(
		&a.ID, &kategorijaID, &sifra, &barkod, &a.Naziv, &a.Opis, &a.Tip, &a.JedinicaMere,
		&a.Kolicina, &a.KolicinMin, &a.Lokacija,
		&a.NabavnaCena, &a.ProdajnaCena, &a.PdvStopa, &marza, &a.Napomena, &a.DatumUnosa, &arhiviran,
	)
	if err != nil {
		return nil, fmt.Errorf("ntech: ArtikalRepo.DohvatiID: %w", err)
	}
	a.Arhiviran = arhiviran == 1

	if kategorijaID.Valid {
		a.KategorijaID = &kategorijaID.Int64
	}
	if sifra.Valid {
		a.Sifra = sifra.String
	}
	if barkod.Valid {
		a.Barkod = barkod.String
	}
	if marza.Valid {
		a.Marza = &marza.Float64
	}

	return &a, nil
}

// Kreiraj dodaje novi artikal u bazu
func (r *ArtikalRepo) Kreiraj(ctx context.Context, a *model.Artikal) (int64, error) {
	var sifra, barkod any
	if a.Sifra != "" {
		sifra = a.Sifra
	}
	if a.Barkod != "" {
		barkod = a.Barkod
	}

	rezultat, err := r.db.ExecContext(ctx, `
		INSERT INTO artikli
			(kategorija_id, sifra, barkod, naziv, opis, tip, jedinica_mere, kolicina, kolicina_min, lokacija,
			 nabavna_cena, prodajna_cena, pdv_stopa, marza, napomena)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.KategorijaID, sifra, barkod, a.Naziv, a.Opis, a.Tip, a.JedinicaMere, a.Kolicina, a.KolicinMin,
		a.Lokacija, a.NabavnaCena, a.ProdajnaCena, a.PdvStopa, a.Marza, a.Napomena,
	)
	if err != nil {
		return 0, fmt.Errorf("ntech: ArtikalRepo.Kreiraj: %w", err)
	}

	id, err := rezultat.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("ntech: ArtikalRepo.Kreiraj: last insert id: %w", err)
	}

	return id, nil
}

// Izmeni ažurira postojeći artikal
func (r *ArtikalRepo) Izmeni(ctx context.Context, a *model.Artikal) error {
	var sifra, barkod any
	if a.Sifra != "" {
		sifra = a.Sifra
	}
	if a.Barkod != "" {
		barkod = a.Barkod
	}

	_, err := r.db.ExecContext(ctx, `
		UPDATE artikli SET
			kategorija_id = ?, sifra = ?, barkod = ?, naziv = ?, opis = ?, tip = ?, jedinica_mere = ?,
			kolicina = ?, kolicina_min = ?, lokacija = ?,
			nabavna_cena = ?, prodajna_cena = ?, pdv_stopa = ?, marza = ?, napomena = ?
		WHERE id = ?`,
		a.KategorijaID, sifra, barkod, a.Naziv, a.Opis, a.Tip, a.JedinicaMere,
		a.Kolicina, a.KolicinMin, a.Lokacija,
		a.NabavnaCena, a.ProdajnaCena, a.PdvStopa, a.Marza, a.Napomena, a.ID,
	)
	if err != nil {
		return fmt.Errorf("ntech: ArtikalRepo.Izmeni: %w", err)
	}

	return nil
}

// SledecaSifra vraća predlog sledeće auto-šifre u formatu PREFIKS-NNNN.
// Prefiks je kôd kategorije (npr. KOMP) ili "ART" ako kategorija nema kôd ili nije zadata.
// Brojač je najveći postojeći broj za taj prefiks + 1 (otporno na brisanje artikala).
func (r *ArtikalRepo) SledecaSifra(ctx context.Context, kategorijaID *int64) (string, error) {
	prefiks := "ART"
	if kategorijaID != nil {
		var kod sql.NullString
		if err := r.db.QueryRowContext(ctx,
			"SELECT kod FROM kategorije WHERE id = ?", *kategorijaID).Scan(&kod); err == nil {
			if kod.Valid && kod.String != "" {
				prefiks = kod.String
			}
		}
	}

	redovi, err := r.db.QueryContext(ctx, "SELECT sifra FROM artikli WHERE sifra LIKE ?", prefiks+"-%")
	if err != nil {
		return "", fmt.Errorf("ntech: ArtikalRepo.SledecaSifra: %w", err)
	}
	defer redovi.Close()

	maxBroj := 0
	for redovi.Next() {
		var s string
		if err := redovi.Scan(&s); err != nil {
			continue
		}
		i := strings.LastIndex(s, "-")
		if i < 0 {
			continue
		}
		if n, err := strconv.Atoi(s[i+1:]); err == nil && n > maxBroj {
			maxBroj = n
		}
	}

	return fmt.Sprintf("%s-%04d", prefiks, maxBroj+1), nil
}

// AzurirajCene menja samo nabavnu i prodajnu cenu artikla (kalkulacija pri prijemu robe).
func (r *ArtikalRepo) AzurirajCene(ctx context.Context, id int64, nabavna, prodajna float64) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE artikli SET nabavna_cena = ?, prodajna_cena = ? WHERE id = ?", nabavna, prodajna, id)
	if err != nil {
		return fmt.Errorf("ntech: ArtikalRepo.AzurirajCene: %w", err)
	}
	return nil
}

// PremestiKategoriju menja samo kategoriju artikla (premeštanje u drugu kategoriju).
// kategorijaID može biti nil — tada artikal ostaje bez kategorije.
func (r *ArtikalRepo) PremestiKategoriju(ctx context.Context, id int64, kategorijaID *int64) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE artikli SET kategorija_id = ? WHERE id = ?", kategorijaID, id)
	if err != nil {
		return fmt.Errorf("ntech: ArtikalRepo.PremestiKategoriju: %w", err)
	}
	return nil
}

// Obrisi briše artikal po ID-u. Ako je artikal u prometu (FK RESTRICT), vraća
// db.ErrArtikalUUpotrebi kako bi pozivalac mogao da ga arhivira umesto da ga obriše.
func (r *ArtikalRepo) Obrisi(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM artikli WHERE id = ?", id)
	if err != nil {
		// SQLITE_CONSTRAINT_FOREIGNKEY (787) — artikal je referenciran iz druge tabele
		var sqliteErr *mosqlite.Error
		if errors.As(err, &sqliteErr) && sqliteErr.Code() == 787 {
			return db.ErrArtikalUUpotrebi
		}
		return fmt.Errorf("ntech: ArtikalRepo.Obrisi: %w", err)
	}

	return nil
}

// Arhiviraj označava artikal kao arhiviran (soft delete za artikle u prometu)
func (r *ArtikalRepo) Arhiviraj(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "UPDATE artikli SET arhiviran = 1 WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("ntech: ArtikalRepo.Arhiviraj: %w", err)
	}
	return nil
}

// Vrati poništava arhiviranje i vraća artikal u aktivnu listu
func (r *ArtikalRepo) Vrati(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "UPDATE artikli SET arhiviran = 0 WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("ntech: ArtikalRepo.Vrati: %w", err)
	}
	return nil
}

// DobavljaciArtikla vraća ID-jeve dobavljača vezanih za artikal
func (r *ArtikalRepo) DobavljaciArtikla(ctx context.Context, artikalID int64) ([]int64, error) {
	redovi, err := r.db.QueryContext(ctx,
		"SELECT dobavljac_id FROM artikal_dobavljac WHERE artikal_id = ? ORDER BY dobavljac_id", artikalID)
	if err != nil {
		return nil, fmt.Errorf("ntech: ArtikalRepo.DobavljaciArtikla: %w", err)
	}
	defer redovi.Close()

	var ids []int64
	for redovi.Next() {
		var id int64
		if err := redovi.Scan(&id); err != nil {
			return nil, fmt.Errorf("ntech: ArtikalRepo.DobavljaciArtikla: scan: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// PostaviDobavljaceArtikla zamenjuje skup dobavljača artikla datim ID-jevima (u transakciji)
func (r *ArtikalRepo) PostaviDobavljaceArtikla(ctx context.Context, artikalID int64, dobavljaciID []int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ntech: ArtikalRepo.PostaviDobavljaceArtikla: begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "DELETE FROM artikal_dobavljac WHERE artikal_id = ?", artikalID); err != nil {
		return fmt.Errorf("ntech: ArtikalRepo.PostaviDobavljaceArtikla: delete: %w", err)
	}
	for _, did := range dobavljaciID {
		if _, err := tx.ExecContext(ctx,
			"INSERT OR IGNORE INTO artikal_dobavljac (artikal_id, dobavljac_id) VALUES (?, ?)", artikalID, did); err != nil {
			return fmt.Errorf("ntech: ArtikalRepo.PostaviDobavljaceArtikla: insert: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("ntech: ArtikalRepo.PostaviDobavljaceArtikla: commit: %w", err)
	}
	return nil
}

// PoveziDobavljaca dodaje vezu artikal–dobavljač ako ne postoji (auto pri nabavci)
func (r *ArtikalRepo) PoveziDobavljaca(ctx context.Context, artikalID, dobavljacID int64) error {
	_, err := r.db.ExecContext(ctx,
		"INSERT OR IGNORE INTO artikal_dobavljac (artikal_id, dobavljac_id) VALUES (?, ?)", artikalID, dobavljacID)
	if err != nil {
		return fmt.Errorf("ntech: ArtikalRepo.PoveziDobavljaca: %w", err)
	}
	return nil
}

// OdveziDobavljaca uklanja vezu artikal–dobavljač
func (r *ArtikalRepo) OdveziDobavljaca(ctx context.Context, artikalID, dobavljacID int64) error {
	_, err := r.db.ExecContext(ctx,
		"DELETE FROM artikal_dobavljac WHERE artikal_id = ? AND dobavljac_id = ?", artikalID, dobavljacID)
	if err != nil {
		return fmt.Errorf("ntech: ArtikalRepo.OdveziDobavljaca: %w", err)
	}
	return nil
}

// SveDobavljaceArtikala vraća mapu artikal_id → lista dobavljac_id
func (r *ArtikalRepo) SveDobavljaceArtikala(ctx context.Context) (map[int64][]int64, error) {
	redovi, err := r.db.QueryContext(ctx,
		"SELECT artikal_id, dobavljac_id FROM artikal_dobavljac ORDER BY artikal_id")
	if err != nil {
		return nil, fmt.Errorf("ntech: ArtikalRepo.SveDobavljaceArtikala: %w", err)
	}
	defer redovi.Close()

	mapa := make(map[int64][]int64)
	for redovi.Next() {
		var aid, did int64
		if err := redovi.Scan(&aid, &did); err != nil {
			return nil, fmt.Errorf("ntech: ArtikalRepo.SveDobavljaceArtikala: scan: %w", err)
		}
		mapa[aid] = append(mapa[aid], did)
	}
	return mapa, nil
}

// KorigujKolicinu postavlja novu količinu i upisuje korekciju u magacinske_promene
func (r *ArtikalRepo) KorigujKolicinu(ctx context.Context, artikalID int64, novaKolicina int, korisnikID *int64, napomena string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ntech: ArtikalRepo.KorigujKolicinu: begin: %w", err)
	}
	defer tx.Rollback()

	var staraCena float64
	var staraKolicina int
	err = tx.QueryRowContext(ctx, "SELECT kolicina, nabavna_cena FROM artikli WHERE id = ?", artikalID).
		Scan(&staraKolicina, &staraCena)
	if err != nil {
		return fmt.Errorf("ntech: ArtikalRepo.KorigujKolicinu: dohvati: %w", err)
	}

	if _, err = tx.ExecContext(ctx, "UPDATE artikli SET kolicina = ? WHERE id = ?", novaKolicina, artikalID); err != nil {
		return fmt.Errorf("ntech: ArtikalRepo.KorigujKolicinu: update: %w", err)
	}

	promena := novaKolicina - staraKolicina
	if err = zabeleziMagacinPromenu(ctx, tx, artikalID, model.PromenaKorekcija, promena,
		staraKolicina, novaKolicina, 0, korisnikID, napomena); err != nil {
		return fmt.Errorf("ntech: ArtikalRepo.KorigujKolicinu: %w", err)
	}

	return tx.Commit()
}

// PrebrojiPoFilteru vraća ukupan broj artikala koji zadovoljavaju filter (bez LIMIT/OFFSET)
func (r *ArtikalRepo) PrebrojiPoFilteru(ctx context.Context, filter db.ArtikalFilter) (int, error) {
	upit := `
		SELECT COUNT(*)
		FROM artikli a
		LEFT JOIN kategorije k ON a.kategorija_id = k.id
		WHERE 1=1`

	args := []any{}

	if filter.Pretraga != "" {
		upit += " AND (a.naziv LIKE ? OR a.sifra LIKE ? OR a.barkod LIKE ? OR a.lokacija LIKE ? OR k.naziv LIKE ?)"
		t := "%" + filter.Pretraga + "%"
		args = append(args, t, t, t, t, t)
	}

	if filter.KategorijaID != nil {
		upit += " AND a.kategorija_id = ?"
		args = append(args, *filter.KategorijaID)
	}

	if filter.Tip != "" {
		upit += " AND a.tip = ?"
		args = append(args, filter.Tip)
	}

	if filter.SamoKriticni {
		upit += " AND (a.tip = 'proizvod' OR a.tip = '') AND a.kolicina <= a.kolicina_min"
	}

	if filter.Arhivirani {
		upit += " AND a.arhiviran = 1"
	} else {
		upit += " AND a.arhiviran = 0"
	}

	var broj int
	if err := r.db.QueryRowContext(ctx, upit, args...).Scan(&broj); err != nil {
		return 0, fmt.Errorf("ntech: ArtikalRepo.PrebrojiPoFilteru: %w", err)
	}

	return broj, nil
}
