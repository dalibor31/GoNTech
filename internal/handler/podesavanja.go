package handler

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"encoding/json"

	"ntech/internal/auth"
	"ntech/internal/db"
	"ntech/internal/db/sqlite"
	"ntech/internal/middleware"
	"ntech/internal/model"
)

// PostaviDemoKorisnika osigurava da postoji korisnik "Demo" sa ulogom "admin"
// i podrazumevanom lozinkom. Poziva se pri startu demo instance, i ponovo
// posle uvoza/vraćanja baze (jer taj hot-swap donosi svoje korisnike iz
// uvezenog/vraćenog fajla, koji mogu imati drugačiju ili nepoznatu lozinku).
func PostaviDemoKorisnika(ctx context.Context, repo db.KorisniciRepository) error {
	const (
		demoIme     = "Demo"
		demoLozinka = "Demo1234"
	)
	hash, err := auth.HashujLozinku(demoLozinka)
	if err != nil {
		return fmt.Errorf("ntech: PostaviDemoKorisnika: %w", err)
	}
	korisnik, err := repo.DohvatiPoImenu(ctx, demoIme)
	if err != nil {
		// korisnik ne postoji — kreiraj ga
		if _, err := repo.Kreiraj(ctx, demoIme, hash, "admin"); err != nil {
			return fmt.Errorf("ntech: PostaviDemoKorisnika: kreiranje: %w", err)
		}
		slog.Info("demo korisnik kreiran", "korisnik", demoIme)
		return nil
	}
	// korisnik postoji — resetuj lozinku i osiguraj da je aktivan
	if err := repo.PromeniLozinku(ctx, korisnik.ID, hash); err != nil {
		return fmt.Errorf("ntech: PostaviDemoKorisnika: lozinka: %w", err)
	}
	if !korisnik.Aktivan {
		if err := repo.AzurirajAktivan(ctx, korisnik.ID, true); err != nil {
			return fmt.Errorf("ntech: PostaviDemoKorisnika: aktivan: %w", err)
		}
	}
	slog.Info("demo korisnik resetovan", "korisnik", demoIme)
	return nil
}

// lokalniKljuceviPodesavanja su ključevi iz tabele podesavanja koji opisuju
// OVU instalaciju/server (izgled login ekrana, adresa fiskalnog uređaja,
// raspored backupa...), a ne podatke firme. Pri uvozu ili vraćanju baze sa
// drugog računara ovi ključevi se NE prepisuju uvezenim vrednostima — server
// zadržava svoje. Sve ostalo u podesavanja smatra se podacima firme i putuje
// sa uvezenom/vraćenom bazom.
var lokalniKljuceviPodesavanja = []string{
	"backup_interval_sati",
	"backup_broj_kopija",
	"login_pozadina",
	"login_pozadina_opacity",
	"login_pozadina_blur_pozadine",
	"login_pozadina_blur_kartice",
	"login_pozadina_zatamnjenje_kartice",
	"topbar_logo_slika",
	"topbar_logo_tekst",
	"pfr_url",
	"pfr_tip",
	"qr_bazni_url",
	"verify_host",
}

// ucitajLokalnaPodesavanja čita trenutne vrednosti lokalnih ključeva iz baze
// pre nego što se baza zameni uvozom/vraćanjem backupa.
func ucitajLokalnaPodesavanja(ctx context.Context, db *sql.DB) map[string]string {
	vrednosti := make(map[string]string, len(lokalniKljuceviPodesavanja))
	for _, kljuc := range lokalniKljuceviPodesavanja {
		var vrednost string
		if err := db.QueryRowContext(ctx, "SELECT vrednost FROM podesavanja WHERE kljuc = ?", kljuc).Scan(&vrednost); err == nil {
			vrednosti[kljuc] = vrednost
		}
	}
	return vrednosti
}

// upisiLokalnaPodesavanja upisuje sačuvane lokalne vrednosti nazad u bazu
// posle zamene, prepisujući preko onoga što je došlo iz uvezene/vraćene baze.
func upisiLokalnaPodesavanja(ctx context.Context, db *sql.DB, vrednosti map[string]string) {
	for kljuc, vrednost := range vrednosti {
		if _, err := db.ExecContext(ctx,
			"INSERT INTO podesavanja (kljuc, vrednost) VALUES (?, ?) ON CONFLICT(kljuc) DO UPDATE SET vrednost = excluded.vrednost",
			kljuc, vrednost); err != nil {
			slog.Error("upis lokalnih podešavanja: greška", "kljuc", kljuc, "error", err)
		}
	}
}

// lokalneTabelePodesavanja su tabele koje opisuju podešavanja OVOG servera, a
// ne poslovne podatke. Korisnici NAMERNO nisu ovde: poslovni podaci (servisni
// nalozi, magacinske promene...) referenciraju konkretne korisnike preko ID-a
// (ko je uradio nalog, ko je izvršio promenu), pa moraju putovati zajedno —
// razdvajanje bi ili pokidalo FK ili tiho pripisalo tuđe podatke pogrešnoj
// osobi. Zato korisnici putuju sa uvezenom/vraćenom bazom kao i ostali
// poslovni podaci; samo dozvole (permisije po ulozi, bez FK ka korisnicima)
// ostaju lokalne.
var lokalneTabelePodesavanja = []string{
	"dozvole",
}

// volatilneTabeleBezbednosti su sesije i srodno runtime/bezbednosno stanje
// koje referencira korisnike preko FK. Pošto korisnici putuju sa uvezenom
// bazom (vidi lokalneTabelePodesavanja), staro stanje ovih tabela bi posle
// zamene moglo da referencira nepostojeće ili pogrešne korisnike — zato se
// jednostavno brišu (ne vraćaju iz snimka), što tera sve na ponovnu prijavu.
var volatilneTabeleBezbednosti = []string{
	"webauthn_kredencijali",
	"rezervni_kodovi",
	"login_istorija",
	"sesije",
	"pokusaji_prijave",
}

// ocistiVolatilneTabele briše sesije i srodno bezbednosno stanje posle zamene
// baze — vidi volatilneTabeleBezbednosti.
func ocistiVolatilneTabele(ctx context.Context, db *sql.DB) {
	for _, tabela := range volatilneTabeleBezbednosti {
		if _, err := db.ExecContext(ctx, "DELETE FROM "+tabela); err != nil {
			slog.Error("čišćenje volatilnih tabela: greška", "tabela", tabela, "error", err)
		}
	}
}

// tabelaSnimak čuva sve redove jedne tabele (generički, bez fiksne šeme) radi
// vraćanja posle zamene baze.
type tabelaSnimak struct {
	kolone []string
	redovi [][]any
}

// ucitajTabelu učitava sve redove i imena kolona zadate tabele.
func ucitajTabelu(ctx context.Context, db *sql.DB, tabela string) ([]string, [][]any, error) {
	rows, err := db.QueryContext(ctx, "SELECT * FROM "+tabela)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	kolone, err := rows.Columns()
	if err != nil {
		return nil, nil, err
	}

	var redovi [][]any
	for rows.Next() {
		vrednosti := make([]any, len(kolone))
		pokazivaci := make([]any, len(kolone))
		for i := range vrednosti {
			pokazivaci[i] = &vrednosti[i]
		}
		if err := rows.Scan(pokazivaci...); err != nil {
			return nil, nil, err
		}
		redovi = append(redovi, vrednosti)
	}
	return kolone, redovi, rows.Err()
}

// ucitajLokalneTabele čita trenutni sadržaj naloga/bezbednosnih tabela pre
// zamene baze.
func ucitajLokalneTabele(ctx context.Context, db *sql.DB) map[string]tabelaSnimak {
	snimci := make(map[string]tabelaSnimak, len(lokalneTabelePodesavanja))
	for _, tabela := range lokalneTabelePodesavanja {
		kolone, redovi, err := ucitajTabelu(ctx, db, tabela)
		if err != nil {
			slog.Error("čuvanje lokalnih tabela: greška pri čitanju", "tabela", tabela, "error", err)
			continue
		}
		snimci[tabela] = tabelaSnimak{kolone: kolone, redovi: redovi}
	}
	return snimci
}

// upisiLokalneTabele vraća sačuvani sadržaj naloga/bezbednosnih tabela posle
// zamene baze, brišući ono što je stiglo iz uvezene/vraćene baze.
func upisiLokalneTabele(ctx context.Context, db *sql.DB, snimci map[string]tabelaSnimak) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		slog.Error("upis lokalnih tabela: greška pri otvaranju transakcije", "error", err)
		return
	}
	defer tx.Rollback()

	for i := len(lokalneTabelePodesavanja) - 1; i >= 0; i-- {
		tabela := lokalneTabelePodesavanja[i]
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+tabela); err != nil {
			slog.Error("upis lokalnih tabela: greška pri brisanju", "tabela", tabela, "error", err)
			return
		}
	}

	for _, tabela := range lokalneTabelePodesavanja {
		snimak, ok := snimci[tabela]
		if !ok || len(snimak.redovi) == 0 {
			continue
		}
		upit := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
			tabela, strings.Join(snimak.kolone, ", "),
			strings.TrimSuffix(strings.Repeat("?, ", len(snimak.kolone)), ", "))
		for _, red := range snimak.redovi {
			if _, err := tx.ExecContext(ctx, upit, red...); err != nil {
				slog.Error("upis lokalnih tabela: greška pri upisu", "tabela", tabela, "error", err)
				return
			}
		}
	}

	if err := tx.Commit(); err != nil {
		slog.Error("upis lokalnih tabela: greška pri komitovanju", "error", err)
	}
}

// podrazumevaniUsloviServisa je podrazumevani tekst uslova servisa koji se
// štampa na reversu; korisnik ga može izmeniti u Podešavanja → Servis.
const podrazumevaniUsloviServisa = `1. Servis ne odgovara za podatke na uređaju. Klijent je dužan da sam napravi rezervnu kopiju podataka pre predaje.
2. Uređaj se preuzima uz ovaj revers i ličnu kartu. Bez reversa uređaj se ne izdaje.
3. Rok za podizanje uređaja je 30 dana od obaveštenja o završetku. Po isteku roka servis ne odgovara za uređaj.
4. Ako klijent odustane od popravke, naplaćuje se dijagnostika prema cenovniku.
5. Garancija na izvršeni servis važi za zamenjene delove i obavljeni rad, ne i za nove kvarove.
6. Potpisom klijent potvrđuje da je saglasan sa navedenim uslovima.`

// podrazumevanaKlauzulaPredracuna je podrazumevana napomena koja se štampa na
// dnu predračuna; korisnik je može izmeniti u Podešavanja → Servis.
const podrazumevanaKlauzulaPredracuna = `Procena je data na osnovu zahteva klijenta i nije fiskalni dokument. Ako se tokom rada utvrdi dodatni kvar, servis kontaktira klijenta radi saglasnosti pre nastavka radova i izmene cene.`

// PodaciPodesavanja su podaci za stranicu podešavanja
type PodaciPodesavanja struct {
	model.PodaciStranice
	NazivFirme  string
	Podnazlov   string
	Adresa      string
	Telefon     string
	PIB         string
	MaticniBroj string
	// poslovna jedinica — za fiskalizaciju (oznaka PJ se šalje uz svaki račun)
	NazivPJ         string
	OznakaPJ        string
	Opstina         string
	Grad            string
	LogoPutanja     string
	TopbarLogoSlika bool
	// profil firme — pravni/poreski status (Faza 0); određuje koji se zakonski moduli pale
	FirmaPravniOblik                string
	FirmaPdvObveznik                string
	FirmaFiskalizacija              string
	FirmaRezim                      string
	Sacuvano                        bool
	Verzija                         string
	LogoGreska                      string
	BackupVracen                    bool
	Backupi                         []BackupInfo
	BackupIntervalSati              string
	BackupBrojKopija                string
	KalkulacijaMarza                string
	ServisGarancijaDana             string
	ServisCenaDijagnostike          string
	PredracunRokDana                string
	PredvidjenRokDana               string
	ServisUslovi                    string
	ServisKlauzulaPredracuna        string
	QrBazniUrl                      string
	PfrUrl                          string
	PfrTip                          string
	PfrKasir                        string
	PfrApiKey                       string
	VerifyHost                      string
	BePin                           string
	BeLimit                         string
	BeEnabled                       string
	FiskalPismo                     string
	LoginPozadina                   string
	LoginPozadinaOpacity            string
	LoginPozadinaBlurPozadine       string
	LoginPozadinaBlurKartice        string
	LoginPozadinaZatamnjenjeKartice string
}

// BackupInfo opisuje jedan backup fajl
type BackupInfo struct {
	Ime      string
	Datum    string
	Velicina string
}

// validnoImeBackupa proverava da li je ime backup fajla bezbedno (bez path traversala)
var validnoImeBackupa = regexp.MustCompile(`^ntech_\d{8}_\d{6}\.db$`)

// Podesavanja renderuje stranicu podešavanja
func (h *Handler) Podesavanja(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "podesavanja.pregled"); !ok {
		return
	}
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "podesavanja"
	ps.NaslovStranice = "Podešavanja"
	podaci := PodaciPodesavanja{
		PodaciStranice:                  ps,
		NazivFirme:                      podesavanja["naziv_firme"],
		Podnazlov:                       podesavanja["podnazlov"],
		Adresa:                          podesavanja["adresa"],
		Telefon:                         podesavanja["telefon"],
		PIB:                             podesavanja["pib"],
		MaticniBroj:                     podesavanja["maticni_broj"],
		NazivPJ:                         podesavanja["poslovna_jedinica_naziv"],
		OznakaPJ:                        podesavanja["poslovna_jedinica_oznaka"],
		Opstina:                         podesavanja["opstina"],
		Grad:                            podesavanja["grad"],
		LogoPutanja:                     podesavanja["logo_putanja"],
		TopbarLogoSlika:                 podesavanja["topbar_logo_slika"] == "1",
		Sacuvano:                        r.URL.Query().Get("sacuvano") == "1",
		BackupVracen:                    r.URL.Query().Get("sacuvano") == "vraceno",
		Verzija:                         h.Verzija,
		LogoGreska:                      r.URL.Query().Get("logo_greska"),
		Backupi:                         ucitajListuBackupa(),
		LoginPozadina:                   podesavanja["login_pozadina"],
		LoginPozadinaOpacity:            vrednostIliDefault(podesavanja, "login_pozadina_opacity", "50"),
		LoginPozadinaBlurPozadine:       vrednostIliDefault(podesavanja, "login_pozadina_blur_pozadine", "0"),
		LoginPozadinaBlurKartice:        vrednostIliDefault(podesavanja, "login_pozadina_blur_kartice", "12"),
		LoginPozadinaZatamnjenjeKartice: vrednostIliDefault(podesavanja, "login_pozadina_zatamnjenje_kartice", "0"),
		BackupIntervalSati:              vrednostIliDefault(podesavanja, "backup_interval_sati", "24"),
		BackupBrojKopija:                vrednostIliDefault(podesavanja, "backup_broj_kopija", "7"),
	}

	h.renderujTemplate(w, "podesavanja", podaci)
}

// ucitajListuBackupa vraća sortiranu listu fajlova iz backups/ foldera
func ucitajListuBackupa() []BackupInfo {
	fajlovi, _ := filepath.Glob(filepath.Join("backups", "ntech_*.db"))
	sort.Sort(sort.Reverse(sort.StringSlice(fajlovi)))
	var lista []BackupInfo
	for _, f := range fajlovi {
		info, err := os.Stat(f)
		if err != nil {
			continue
		}
		vel := info.Size()
		var velStr string
		switch {
		case vel >= 1024*1024:
			velStr = fmt.Sprintf("%.1f MB", float64(vel)/(1024*1024))
		default:
			velStr = fmt.Sprintf("%d KB", vel/1024)
		}
		datum := info.ModTime().Format("02.01.2006. 15:04:05")
		lista = append(lista, BackupInfo{
			Ime:      filepath.Base(f),
			Datum:    datum,
			Velicina: velStr,
		})
	}
	return lista
}

// vrednostIliDefault vraća vrednost iz mape ako postoji i nije prazan string, inače vraća podrazumevanu vrednost
func vrednostIliDefault(m map[string]string, kljuc, podrazumevano string) string {
	if v := m[kljuc]; v != "" {
		return v
	}
	return podrazumevano
}

// VratiBackup zamenjuje trenutnu bazu sa izabranim backup fajlom
func (h *Handler) VratiBackup(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "backup.pokreni"); !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/podesavanja?backup_greska=Greška+pri+čitanju+zahteva", http.StatusSeeOther)
		return
	}

	ime := r.FormValue("ime")
	if !validnoImeBackupa.MatchString(ime) {
		http.Redirect(w, r, "/podesavanja?backup_greska=Neispravan+naziv+fajla", http.StatusSeeOther)
		return
	}

	putanjaBackupa := filepath.Join("backups", ime)
	if _, err := os.Stat(putanjaBackupa); err != nil {
		http.Redirect(w, r, "/podesavanja?backup_greska=Backup+fajl+nije+pronađen", http.StatusSeeOther)
		return
	}

	// pre obnove, sačuvaj trenutno stanje baze
	sigurnosni := filepath.Join("backups", fmt.Sprintf("ntech_%s_pred_vracanjem.db", time.Now().Format("20060102_150405")))
	if _, err := h.DB.ExecContext(r.Context(), "VACUUM INTO ?", sigurnosni); err != nil {
		slog.Error("vrati backup: greška pri kreiranju sigurnosne kopije", "error", err)
		http.Redirect(w, r, "/podesavanja?backup_greska=Greška+pri+kreiranju+sigurnosne+kopije", http.StatusSeeOther)
		return
	}

	// isprazni WAL u glavni fajl
	if _, err := h.DB.ExecContext(r.Context(), "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		slog.Error("vrati backup: wal_checkpoint greška", "error", err)
	}

	lokalnaPodesavanja := ucitajLokalnaPodesavanja(r.Context(), h.DB)
	lokalneTabele := ucitajLokalneTabele(r.Context(), h.DB)

	// Sama zamena baze (zatvaranje stare, kopiranje, otvaranje nove) radi se u
	// zasebnoj gorutini pod EKSKLUZIVNIM zaključavanjem (h.mu.Lock). Razlog: ovaj
	// zahtev još drži deljeno zaključavanje (ZakljucajCitanje middleware), pa bi
	// uzimanje ekskluzivnog u istoj gorutini izazvalo deadlock. Ekskluzivno
	// zaključavanje sačeka da svi tekući zahtevi (uključujući ovaj, čim vrati
	// odgovor) završe, pa tek onda menja konekciju — bez data race-a i bez upita
	// nad zatvorenom bazom. Sledeći zahtev (redirect na /podesavanja) prirodno
	// sačeka da zamena završi jer čeka na deljeno zaključavanje.
	go func() {
		h.mu.Lock()
		defer h.mu.Unlock()

		if err := h.DB.Close(); err != nil {
			slog.Error("vrati backup: greška pri zatvaranju baze", "error", err)
		}
		if err := kopiraFajl(putanjaBackupa, h.PutanjaBaze); err != nil {
			slog.Error("vrati backup: greška pri kopiranju (baza je zatvorena, potreban restart)", "error", err)
			return
		}
		os.Remove(h.PutanjaBaze + "-wal")
		os.Remove(h.PutanjaBaze + "-shm")

		novaDB, err := sqlite.OtvoriDB(h.PutanjaBaze)
		if err != nil {
			slog.Error("vrati backup: greška pri otvaranju nove baze (potreban restart)", "error", err)
			return
		}
		upisiLokalnaPodesavanja(context.Background(), novaDB, lokalnaPodesavanja)
		upisiLokalneTabele(context.Background(), novaDB, lokalneTabele)
		ocistiVolatilneTabele(context.Background(), novaDB)
		h.reinicijalizujRepozitorijume(novaDB)
		if h.JelDemo {
			if err := PostaviDemoKorisnika(context.Background(), h.KorisniciRepo); err != nil {
				slog.Error("vrati backup: greška pri resetovanju demo korisnika", "error", err)
			}
		}
		slog.Info("baza uspešno obnovljena", "izvor", ime)
	}()

	http.Redirect(w, r, "/podesavanja?sacuvano=vraceno", http.StatusSeeOther)
}

// kopiraFajl kopira fajl sa izvora na odredište
func kopiraFajl(izvor, odrediste string) error {
	src, err := os.Open(izvor)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(odrediste)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}

// UvezizBazu zamenjuje trenutnu bazu fajlom koji korisnik otpremi sa svog računara
func (h *Handler) UvezizBazu(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "backup.pokreni"); !ok {
		return
	}

	const maxVelicinaUvoza = 500 << 20 // 500 MB
	r.Body = http.MaxBytesReader(w, r.Body, maxVelicinaUvoza)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Redirect(w, r, "/podesavanja?backup_greska=Fajl+je+prevelik+ili+zahtev+nije+ispravan", http.StatusSeeOther)
		return
	}

	fajl, _, err := r.FormFile("baza")
	if err != nil {
		http.Redirect(w, r, "/podesavanja?backup_greska=Fajl+nije+izabran", http.StatusSeeOther)
		return
	}
	defer fajl.Close()

	zaglavlje := make([]byte, 16)
	if _, err := io.ReadFull(fajl, zaglavlje); err != nil || string(zaglavlje) != "SQLite format 3\x00" {
		http.Redirect(w, r, "/podesavanja?backup_greska=Fajl+nije+ispravna+SQLite+baza", http.StatusSeeOther)
		return
	}

	privremena := filepath.Join("backups", fmt.Sprintf("uvoz_%s.db", time.Now().Format("20060102_150405")))
	odrediste, err := os.Create(privremena)
	if err != nil {
		slog.Error("uvezi bazu: greška pri kreiranju privremenog fajla", "error", err)
		http.Redirect(w, r, "/podesavanja?backup_greska=Greška+pri+čuvanju+otpremljenog+fajla", http.StatusSeeOther)
		return
	}
	if _, err := odrediste.Write(zaglavlje); err != nil {
		odrediste.Close()
		http.Redirect(w, r, "/podesavanja?backup_greska=Greška+pri+čuvanju+otpremljenog+fajla", http.StatusSeeOther)
		return
	}
	if _, err := io.Copy(odrediste, fajl); err != nil {
		odrediste.Close()
		slog.Error("uvezi bazu: greška pri čuvanju otpremljenog fajla", "error", err)
		http.Redirect(w, r, "/podesavanja?backup_greska=Greška+pri+čuvanju+otpremljenog+fajla", http.StatusSeeOther)
		return
	}
	odrediste.Close()

	// pre uvoza, sačuvaj trenutno stanje baze
	sigurnosni := filepath.Join("backups", fmt.Sprintf("ntech_%s_pred_uvozom.db", time.Now().Format("20060102_150405")))
	if _, err := h.DB.ExecContext(r.Context(), "VACUUM INTO ?", sigurnosni); err != nil {
		slog.Error("uvezi bazu: greška pri kreiranju sigurnosne kopije", "error", err)
		os.Remove(privremena)
		http.Redirect(w, r, "/podesavanja?backup_greska=Greška+pri+kreiranju+sigurnosne+kopije", http.StatusSeeOther)
		return
	}

	// isprazni WAL u glavni fajl
	if _, err := h.DB.ExecContext(r.Context(), "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		slog.Error("uvezi bazu: wal_checkpoint greška", "error", err)
	}

	lokalnaPodesavanja := ucitajLokalnaPodesavanja(r.Context(), h.DB)
	lokalneTabele := ucitajLokalneTabele(r.Context(), h.DB)

	// zamena baze radi se pod ekskluzivnim zaključavanjem — isti obrazac kao VratiBackup
	go func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		defer os.Remove(privremena)

		if err := h.DB.Close(); err != nil {
			slog.Error("uvezi bazu: greška pri zatvaranju baze", "error", err)
		}
		if err := kopiraFajl(privremena, h.PutanjaBaze); err != nil {
			slog.Error("uvezi bazu: greška pri kopiranju (baza je zatvorena, potreban restart)", "error", err)
			return
		}
		os.Remove(h.PutanjaBaze + "-wal")
		os.Remove(h.PutanjaBaze + "-shm")

		novaDB, err := sqlite.OtvoriDB(h.PutanjaBaze)
		if err != nil {
			slog.Error("uvezi bazu: greška pri otvaranju nove baze (potreban restart)", "error", err)
			return
		}
		upisiLokalnaPodesavanja(context.Background(), novaDB, lokalnaPodesavanja)
		upisiLokalneTabele(context.Background(), novaDB, lokalneTabele)
		ocistiVolatilneTabele(context.Background(), novaDB)
		h.reinicijalizujRepozitorijume(novaDB)
		if h.JelDemo {
			if err := PostaviDemoKorisnika(context.Background(), h.KorisniciRepo); err != nil {
				slog.Error("uvezi bazu: greška pri resetovanju demo korisnika", "error", err)
			}
		}
		slog.Info("baza uspešno uvezena")
	}()

	http.Redirect(w, r, "/podesavanja?sacuvano=vraceno", http.StatusSeeOther)
}

// validirajMaticniBroj proverava da li uneti matični broj ima tačno 8 cifara.
// Prazan broj je dozvoljen (polje nije obavezno). Vraća srpsku poruku o grešci
// ili prazan string ako je broj ispravan.
func validirajMaticniBroj(broj string) string {
	if broj == "" {
		return ""
	}
	if len(broj) != 8 {
		return "Matični broj mora imati tačno 8 cifara."
	}
	for _, c := range broj {
		if c < '0' || c > '9' {
			return "Matični broj sme sadržati samo cifre."
		}
	}
	return ""
}

// validirajProfilFirme proverava da li je kombinacija pravnog/poreskog statusa
// pravno moguća — vidi docs/Greške.md §1. „Samo evidencija” gasi ceo zakonski
// sloj, pa dok je firma u tom režimu ne postoji osnov da bude PDV obveznik
// niti da izdaje fiskalne račune.
func validirajProfilFirme(rezim, pdvObveznik, fiskalizacija string) string {
	if rezim != "samo_evidencija" {
		return ""
	}
	if pdvObveznik == "da" || fiskalizacija == "da" {
		return "Režim „Samo evidencija” ne dozvoljava PDV obveznika ni izdavanje fiskalnih računa — prebacite firmu na režim „Pun” ako stvarno posluje."
	}
	return ""
}

// SacuvajPodesavanja prima POST i čuva podešavanja u bazu
func (h *Handler) SacuvajPodesavanja(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "podesavanja.izmeni"); !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	// checkbox „topbar_logo_slika" živi u Firma sekciji. Pošto se sekcije auto-snimaju
	// odvojeno, čitamo ga SAMO kad je poslata Firma sekcija (marker `_sekcija_firma`),
	// inače ostaje "" pa ga petlja ispod preskoči i ne dira postojeću vrednost.
	// (Checkbox koji nije čekiran se ne šalje, pa bi inače snimanje druge sekcije
	// ugasilo logo u topbaru.)
	topbarLogoSlika := ""
	if _, jeFirmaSekcija := r.Form["_sekcija_firma"]; jeFirmaSekcija {
		topbarLogoSlika = "0"
		if r.FormValue("topbar_logo_slika") == "1" {
			topbarLogoSlika = "1"
		}
	}

	// matični broj — 8 cifara; prazno je dozvoljeno (polje nije obavezno)
	maticniBroj := strings.TrimSpace(r.FormValue("maticni_broj"))
	if greska := validirajMaticniBroj(maticniBroj); greska != "" {
		middleware.SetFlash(w, r, h.DB, "greska", greska)
		sledeci := "/podesavanja"
		if r.FormValue("_next") == "/admin/podesavanja/opste" {
			sledeci = "/admin/podesavanja/opste"
		}
		http.Redirect(w, r, sledeci, http.StatusSeeOther)
		return
	}

	if _, jeFirmaSekcija := r.Form["_sekcija_firma"]; jeFirmaSekcija {
		greska := validirajProfilFirme(r.FormValue("firma_rezim"), r.FormValue("firma_pdv_obveznik"), r.FormValue("firma_fiskalizacija"))
		if greska != "" {
			middleware.SetFlash(w, r, h.DB, "greska", greska)
			sledeci := "/podesavanja"
			if r.FormValue("_next") == "/admin/podesavanja/opste" {
				sledeci = "/admin/podesavanja/opste"
			}
			http.Redirect(w, r, sledeci, http.StatusSeeOther)
			return
		}
	}

	polja := map[string]string{
		"naziv_firme":              r.FormValue("naziv_firme"),
		"podnazlov":                r.FormValue("podnazlov"),
		"adresa":                   r.FormValue("adresa"),
		"telefon":                  r.FormValue("telefon"),
		"pib":                      r.FormValue("pib"),
		"maticni_broj":             maticniBroj,
		"poslovna_jedinica_naziv":  r.FormValue("poslovna_jedinica_naziv"),
		"poslovna_jedinica_oznaka": r.FormValue("poslovna_jedinica_oznaka"),
		"opstina":                  r.FormValue("opstina"),
		"grad":                     r.FormValue("grad"),
		"topbar_logo_slika":        topbarLogoSlika,
		// profil firme (Faza 0) — radio dugmad uvek šalju vrednost, pa se uredno čuvaju
		"firma_pravni_oblik":  r.FormValue("firma_pravni_oblik"),
		"firma_pdv_obveznik":  r.FormValue("firma_pdv_obveznik"),
		"firma_fiskalizacija": r.FormValue("firma_fiskalizacija"),
		"firma_rezim":         r.FormValue("firma_rezim"),
		// fiskalizacija — L-PFR i kartica emulator podešavanja
		"pfr_url":        r.FormValue("pfr_url"),
		"pfr_tip":        r.FormValue("pfr_tip"),
		"pfr_kasir":      r.FormValue("pfr_kasir"),
		"verify_host":    r.FormValue("verify_host"),
		"pfr_api_key":    r.FormValue("pfr_api_key"),
		"be_pin":         r.FormValue("be_pin"),
		"be_limit":       r.FormValue("be_limit"),
		"be_enabled":     r.FormValue("be_enabled"),
		"fiskalni_pismo": r.FormValue("fiskalni_pismo"),
	}

	for kljuc, vrednost := range polja {
		if vrednost == "" {
			continue
		}
		if err := sqlite.SacuvajPodesavanje(r.Context(), h.DB, kljuc, vrednost); err != nil {
			http.Error(w, "Greška pri čuvanju podešavanja", http.StatusInternalServerError)
			return
		}
	}

	// whitelist dozvoljenih redirekcija — vrednost uvek dolazi iz mape, nikad od korisnika
	dozvoljeniSledeci := map[string]string{
		"/admin/podesavanja/opste":           "/admin/podesavanja/opste",
		"/admin/podesavanja/sistem":          "/admin/podesavanja/sistem",
		"/admin/podesavanja/servis":          "/admin/podesavanja/servis",
		"/admin/podesavanja/kalkulacija-pdv": "/admin/podesavanja/kalkulacija-pdv",
		"/admin/podesavanja/fiskalizacija":   "/admin/podesavanja/fiskalizacija",
		"/podesavanja":                       "/podesavanja",
	}
	sledeci := "/podesavanja"
	if v, ok := dozvoljeniSledeci[r.FormValue("_next")]; ok {
		sledeci = v
	}

	// backup podešavanja — pri neispravnom unosu javljamo jasnu grešku
	// umesto da ga tiho preskočimo a korisniku prikažemo "sačuvano"
	if v := r.FormValue("backup_interval_sati"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 720 {
			middleware.SetFlash(w, r, h.DB, "greska", "Razmak između backupa mora biti broj između 1 i 720 sati.")
			http.Redirect(w, r, sledeci, http.StatusSeeOther)
			return
		}
		if err := sqlite.SacuvajPodesavanje(r.Context(), h.DB, "backup_interval_sati", strconv.Itoa(n)); err != nil {
			http.Error(w, "Greška pri čuvanju podešavanja", http.StatusInternalServerError)
			return
		}
	}
	if v := r.FormValue("backup_broj_kopija"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 100 {
			middleware.SetFlash(w, r, h.DB, "greska", "Broj kopija mora biti broj između 1 i 100.")
			http.Redirect(w, r, sledeci, http.StatusSeeOther)
			return
		}
		if err := sqlite.SacuvajPodesavanje(r.Context(), h.DB, "backup_broj_kopija", strconv.Itoa(n)); err != nil {
			http.Error(w, "Greška pri čuvanju podešavanja", http.StatusInternalServerError)
			return
		}
	}

	// podrazumevana marža za kalkulaciju (procenat, 0–1000)
	if v := strings.TrimSpace(r.FormValue("kalkulacija_marza")); v != "" {
		marza, err := strconv.ParseFloat(strings.Replace(v, ",", ".", 1), 64)
		if err != nil || marza < 0 || marza > 1000 {
			middleware.SetFlash(w, r, h.DB, "greska", "Marža mora biti broj između 0 i 1000.")
			http.Redirect(w, r, sledeci, http.StatusSeeOther)
			return
		}
		if err := sqlite.SacuvajPodesavanje(r.Context(), h.DB, "kalkulacija_marza", strconv.FormatFloat(marza, 'f', -1, 64)); err != nil {
			http.Error(w, "Greška pri čuvanju podešavanja", http.StatusInternalServerError)
			return
		}
	}

	// podrazumevana garancija za servis (dani, 0–3650)
	if v := strings.TrimSpace(r.FormValue("servis_garancija_dana")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || n > 3650 {
			middleware.SetFlash(w, r, h.DB, "greska", "Garancija mora biti broj između 0 i 3650 dana.")
			http.Redirect(w, r, sledeci, http.StatusSeeOther)
			return
		}
		if err := sqlite.SacuvajPodesavanje(r.Context(), h.DB, "servis_garancija_dana", strconv.Itoa(n)); err != nil {
			http.Error(w, "Greška pri čuvanju podešavanja", http.StatusInternalServerError)
			return
		}
	}

	// podrazumevana cena dijagnostike (din, ≥ 0)
	if v := strings.TrimSpace(r.FormValue("servis_cena_dijagnostike")); v != "" {
		cena, err := strconv.ParseFloat(strings.ReplaceAll(v, ",", "."), 64)
		if err != nil || cena < 0 {
			middleware.SetFlash(w, r, h.DB, "greska", "Cena dijagnostike mora biti broj veći ili jednak 0.")
			http.Redirect(w, r, sledeci, http.StatusSeeOther)
			return
		}
		if err := sqlite.SacuvajPodesavanje(r.Context(), h.DB, "servis_cena_dijagnostike", strconv.FormatFloat(cena, 'f', -1, 64)); err != nil {
			http.Error(w, "Greška pri čuvanju podešavanja", http.StatusInternalServerError)
			return
		}
	}

	// rok važenja predračuna u danima (1–90)
	if v := strings.TrimSpace(r.FormValue("predracun_rok_dana")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 90 {
			middleware.SetFlash(w, r, h.DB, "greska", "Rok važenja predračuna mora biti broj između 1 i 90 dana.")
			http.Redirect(w, r, sledeci, http.StatusSeeOther)
			return
		}
		if err := sqlite.SacuvajPodesavanje(r.Context(), h.DB, "predracun_rok_dana", strconv.Itoa(n)); err != nil {
			http.Error(w, "Greška pri čuvanju podešavanja", http.StatusInternalServerError)
			return
		}
	}

	if v := strings.TrimSpace(r.FormValue("predvidjen_rok_dana")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 365 {
			middleware.SetFlash(w, r, h.DB, "greska", "Predviđen rok popravke mora biti broj između 1 i 365 dana.")
			http.Redirect(w, r, sledeci, http.StatusSeeOther)
			return
		}
		if err := sqlite.SacuvajPodesavanje(r.Context(), h.DB, "predvidjen_rok_dana", strconv.Itoa(n)); err != nil {
			http.Error(w, "Greška pri čuvanju podešavanja", http.StatusInternalServerError)
			return
		}
	}

	// uslovi servisa (slobodan tekst, štampa se na reversu); čuva se uvek,
	// pa korisnik može i da skrati ili isprazni tekst
	if _, ima := r.Form["servis_uslovi"]; ima {
		uslovi := strings.TrimSpace(r.FormValue("servis_uslovi"))
		if err := sqlite.SacuvajPodesavanje(r.Context(), h.DB, "servis_uslovi", uslovi); err != nil {
			http.Error(w, "Greška pri čuvanju podešavanja", http.StatusInternalServerError)
			return
		}
	}

	// klauzula na predračunu (slobodan tekst); čuva se uvek
	if _, ima := r.Form["servis_klauzula_predracuna"]; ima {
		klauzula := strings.TrimSpace(r.FormValue("servis_klauzula_predracuna"))
		if err := sqlite.SacuvajPodesavanje(r.Context(), h.DB, "servis_klauzula_predracuna", klauzula); err != nil {
			http.Error(w, "Greška pri čuvanju podešavanja", http.StatusInternalServerError)
			return
		}
	}

	// bazna adresa za QR kod (npr. http://192.168.1.25:3000); prazno → koristi se host iz zahteva
	if _, ima := r.Form["qr_bazni_url"]; ima {
		bazni := strings.TrimSpace(r.FormValue("qr_bazni_url"))
		if err := sqlite.SacuvajPodesavanje(r.Context(), h.DB, "qr_bazni_url", bazni); err != nil {
			http.Error(w, "Greška pri čuvanju podešavanja", http.StatusInternalServerError)
			return
		}
	}

	http.Redirect(w, r, sledeci+"?sacuvano=1", http.StatusSeeOther)
}

// BackupBaze kreira konzistentnu kopiju baze i šalje je kao attachment
func (h *Handler) BackupBaze(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "backup.pregled"); !ok {
		return
	}
	privremeni := fmt.Sprintf("%s/ntech_backup_%s.db", os.TempDir(), time.Now().Format("20060102_150405"))

	if _, err := h.DB.ExecContext(r.Context(), "VACUUM INTO ?", privremeni); err != nil {
		http.Error(w, "Greška pri kreiranju rezervne kopije", http.StatusInternalServerError)
		return
	}
	defer os.Remove(privremeni)

	ime := fmt.Sprintf("ntech_backup_%s.db", time.Now().Format("20060102"))
	w.Header().Set("Content-Disposition", "attachment; filename=\""+ime+"\"")
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, privremeni)
}

// OtpremiLogo prima multipart upload slike loga i čuva je u web/static/uploads/
func (h *Handler) OtpremiLogo(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "podesavanja.izmeni"); !ok {
		return
	}
	// ograničavamo telo zahteva na 2MB + malo za zaglavlja forme
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20+4096)
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		http.Redirect(w, r, "/admin/podesavanja/opste?logo_greska=Fajl+je+prevelik+%28maksimum+2+MB%29", http.StatusSeeOther)
		return
	}

	fajl, zaglavlje, err := r.FormFile("logo")
	if err != nil {
		http.Redirect(w, r, "/admin/podesavanja/opste?logo_greska=Nije+odabran+fajl", http.StatusSeeOther)
		return
	}
	defer fajl.Close()

	// eksplicitna provera veličine (zaglavlje.Size je postavljeno od strane browsera)
	if zaglavlje.Size > 2<<20 {
		http.Redirect(w, r, "/admin/podesavanja/opste?logo_greska=Fajl+je+prevelik+%28maksimum+2+MB%29", http.StatusSeeOther)
		return
	}

	// proveravamo ekstenziju
	ext := strings.ToLower(filepath.Ext(zaglavlje.Filename))
	dozvoljenoExt := map[string]string{
		".png":  "image/png",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".svg":  "image/svg+xml",
	}
	ocekivaniMime, ok := dozvoljenoExt[ext]
	if !ok {
		http.Redirect(w, r, "/admin/podesavanja/opste?logo_greska=Dozvoljeni+formati+su+PNG%2C+JPG+i+SVG", http.StatusSeeOther)
		return
	}

	// za binarne formate proveravamo i stvarni tip fajla (SVG je tekstualni, preskačemo)
	if ext != ".svg" {
		buf := make([]byte, 512)
		n, _ := fajl.Read(buf)
		stvarniMime := http.DetectContentType(buf[:n])
		if !strings.HasPrefix(stvarniMime, ocekivaniMime) {
			http.Redirect(w, r, "/admin/podesavanja/opste?logo_greska=Sadržaj+fajla+ne+odgovara+odabranoj+ekstenziji", http.StatusSeeOther)
			return
		}
		// vraćamo kursor na početak
		if _, err := fajl.Seek(0, io.SeekStart); err != nil {
			http.Redirect(w, r, "/admin/podesavanja/opste?logo_greska=Greška+pri+obradi+fajla", http.StatusSeeOther)
			return
		}
	}

	// brišemo stare logo fajlove
	stari, _ := filepath.Glob("web/static/uploads/logo.*")
	for _, s := range stari {
		os.Remove(s)
	}

	odrediste := "web/static/uploads/logo" + ext
	dst, err := os.Create(odrediste)
	if err != nil {
		slog.Error("upload loga: ne mogu kreirati fajl", "error", err)
		http.Redirect(w, r, "/admin/podesavanja/opste?logo_greska=Greška+pri+čuvanju+fajla", http.StatusSeeOther)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, fajl); err != nil {
		slog.Error("upload loga: greška pri kopiranju", "error", err)
		http.Redirect(w, r, "/admin/podesavanja/opste?logo_greska=Greška+pri+čuvanju+fajla", http.StatusSeeOther)
		return
	}

	// timestamp u URL-u sprečava browser da koristi staru keširanu sliku
	putanja := fmt.Sprintf("/static/uploads/logo%s?v=%d", ext, time.Now().Unix())
	if err := sqlite.SacuvajPodesavanje(r.Context(), h.DB, "logo_putanja", putanja); err != nil {
		slog.Error("upload loga: greška pri čuvanju putanje", "error", err)
		http.Redirect(w, r, "/admin/podesavanja/opste?logo_greska=Greška+pri+čuvanju+podešavanja", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin/podesavanja/opste?sacuvano=1", http.StatusSeeOther)
}

// UkloniLogo briše logo fajl i čisti putanju iz podešavanja
func (h *Handler) UkloniLogo(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "podesavanja.izmeni"); !ok {
		return
	}
	stari, _ := filepath.Glob("web/static/uploads/logo.*")
	for _, s := range stari {
		os.Remove(s)
	}
	if err := sqlite.SacuvajPodesavanje(r.Context(), h.DB, "logo_putanja", ""); err != nil {
		slog.Error("ukloni logo: greška pri čuvanju", "error", err)
	}
	http.Redirect(w, r, "/admin/podesavanja/opste?sacuvano=1", http.StatusSeeOther)
}

// PodesavanjaServis renderuje stranicu sa podešavanjima servisnog modula
func (h *Handler) PodesavanjaServis(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "podesavanja.pregled"); !ok {
		return
	}
	podaci, err := h.napuniPodaciPodesavanja(r, "Podešavanja — Servis")
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}
	podaci.Stranica = "podesavanja-servis"
	h.renderujTemplate(w, "podesavanja_servis", podaci)
}

// generisiImeUploada vraća slučajno hex ime (16 bajtova) sa datom ekstenzijom
func generisiImeUploada(ext string) (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf) + ext, nil
}

// OtpremiLoginPozadinu prima multipart upload slike i čuva je kao pozadinsku sliku login stranice
func (h *Handler) OtpremiLoginPozadinu(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "podesavanja.login_pozadina"); !ok {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 5<<20+4096)
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Fajl je prevelik (maksimum 5 MB).")
		http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
		return
	}

	fajl, zaglavlje, err := r.FormFile("login_pozadina")
	if err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Nije odabran fajl.")
		http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
		return
	}
	defer fajl.Close()

	if zaglavlje.Size > 5<<20 {
		middleware.SetFlash(w, r, h.DB, "greska", "Fajl je prevelik (maksimum 5 MB).")
		http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
		return
	}

	ext := strings.ToLower(filepath.Ext(zaglavlje.Filename))
	dozvoljenoExt := map[string]string{
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".webp": "image/webp",
	}
	ocekivaniMime, ok := dozvoljenoExt[ext]
	if !ok {
		middleware.SetFlash(w, r, h.DB, "greska", "Dozvoljeni formati su JPG, PNG i WebP.")
		http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
		return
	}

	// proveravamo stvarni tip sadržaja (magic bytes)
	buf := make([]byte, 512)
	n, _ := fajl.Read(buf)
	stvarniMime := http.DetectContentType(buf[:n])
	if !strings.HasPrefix(stvarniMime, ocekivaniMime) {
		middleware.SetFlash(w, r, h.DB, "greska", "Sadržaj fajla ne odgovara odabranoj ekstenziji.")
		http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
		return
	}
	if _, err := fajl.Seek(0, io.SeekStart); err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri obradi fajla.")
		http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
		return
	}

	// briše staru pozadinu sa diska ako postoji
	staraPodesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if stara := staraPodesavanja["login_pozadina"]; stara != "" {
		// putanja u bazi je oblika /static/uploads/ime.ext?v=..., izvlačimo samo ime fajla
		deoBezverzije, _, _ := strings.Cut(stara, "?")
		staroIme := filepath.Base(deoBezverzije)
		os.Remove(filepath.Join("web/static/uploads", staroIme))
	}

	novoIme, err := generisiImeUploada(ext)
	if err != nil {
		slog.Error("upload login pozadine: greška pri generisanju imena", "error", err)
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju fajla.")
		http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
		return
	}

	odrediste := filepath.Join("web/static/uploads", novoIme)
	dst, err := os.Create(odrediste)
	if err != nil {
		slog.Error("upload login pozadine: ne mogu kreirati fajl", "error", err)
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju fajla.")
		http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, fajl); err != nil {
		slog.Error("upload login pozadine: greška pri kopiranju", "error", err)
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju fajla.")
		http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
		return
	}

	putanja := fmt.Sprintf("/static/uploads/%s?v=%d", novoIme, time.Now().Unix())
	if err := sqlite.SacuvajPodesavanje(r.Context(), h.DB, "login_pozadina", putanja); err != nil {
		slog.Error("upload login pozadine: greška pri čuvanju putanje", "error", err)
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju podešavanja.")
		http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
		return
	}

	middleware.SetFlash(w, r, h.DB, "uspeh", "Pozadinska slika je uspešno otpremljena.")
	http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
}

// UkloniLoginPozadinu briše pozadinsku sliku login stranice sa diska i iz podešavanja
func (h *Handler) UkloniLoginPozadinu(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "podesavanja.login_pozadina"); !ok {
		return
	}

	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err == nil {
		if stara := podesavanja["login_pozadina"]; stara != "" {
			deoBezverzije, _, _ := strings.Cut(stara, "?")
			staroIme := filepath.Base(deoBezverzije)
			os.Remove(filepath.Join("web/static/uploads", staroIme))
		}
	}

	if err := sqlite.SacuvajPodesavanje(r.Context(), h.DB, "login_pozadina", ""); err != nil {
		slog.Error("ukloni login pozadinu: greška pri čuvanju", "error", err)
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri uklanjanju slike.")
		http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin/podesavanja/izgled?sacuvano=1", http.StatusSeeOther)
}

// SacuvajLoginPozadinaStilove čuva vrednosti zamućenja i prozirnosti pozadine login stranice
func (h *Handler) SacuvajLoginPozadinaStilove(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "podesavanja.login_pozadina"); !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čitanju forme.")
		http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
		return
	}

	blurPozadineStr := r.FormValue("blur_pozadine")
	blurKarticeStr := r.FormValue("blur_kartice")
	opacityStr := r.FormValue("opacity")
	zatamnjenjeKarticeStr := r.FormValue("zatamnjenje_kartice")

	blurPozadineVal, err := strconv.Atoi(blurPozadineStr)
	if err != nil || blurPozadineVal < 0 || blurPozadineVal > 20 {
		middleware.SetFlash(w, r, h.DB, "greska", "Neispravna vrednost zamućenja pozadine.")
		http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
		return
	}
	blurKarticeVal, err := strconv.Atoi(blurKarticeStr)
	if err != nil || blurKarticeVal < 0 || blurKarticeVal > 20 {
		middleware.SetFlash(w, r, h.DB, "greska", "Neispravna vrednost zamućenja kartice.")
		http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
		return
	}
	opacityVal, err := strconv.Atoi(opacityStr)
	if err != nil || opacityVal < 0 || opacityVal > 80 {
		middleware.SetFlash(w, r, h.DB, "greska", "Neispravna vrednost prozirnosti.")
		http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
		return
	}
	zatamnjenjeKarticeVal, err := strconv.Atoi(zatamnjenjeKarticeStr)
	if err != nil || zatamnjenjeKarticeVal < 0 || zatamnjenjeKarticeVal > 80 {
		middleware.SetFlash(w, r, h.DB, "greska", "Neispravna vrednost zatamnjivanja kartice.")
		http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
		return
	}

	for kljuc, vrednost := range map[string]string{
		"login_pozadina_blur_pozadine":       blurPozadineStr,
		"login_pozadina_blur_kartice":        blurKarticeStr,
		"login_pozadina_opacity":             opacityStr,
		"login_pozadina_zatamnjenje_kartice": zatamnjenjeKarticeStr,
	} {
		if err := sqlite.SacuvajPodesavanje(r.Context(), h.DB, kljuc, vrednost); err != nil {
			slog.Error("greška pri čuvanju stila login pozadine", "kljuc", kljuc, "error", err)
			middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju podešavanja.")
			http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
			return
		}
	}

	http.Redirect(w, r, "/admin/podesavanja/izgled?sacuvano=1", http.StatusSeeOther)
}

// napuniPodaciPodesavanja učitava sva podešavanja i kreira strukturu za template
func (h *Handler) napuniPodaciPodesavanja(r *http.Request, naslov string) (PodaciPodesavanja, error) {
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		return PodaciPodesavanja{}, err
	}
	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "podesavanja"
	ps.NaslovStranice = naslov
	return PodaciPodesavanja{
		PodaciStranice:                  ps,
		NazivFirme:                      podesavanja["naziv_firme"],
		Podnazlov:                       podesavanja["podnazlov"],
		Adresa:                          podesavanja["adresa"],
		Telefon:                         podesavanja["telefon"],
		PIB:                             podesavanja["pib"],
		MaticniBroj:                     podesavanja["maticni_broj"],
		NazivPJ:                         podesavanja["poslovna_jedinica_naziv"],
		OznakaPJ:                        podesavanja["poslovna_jedinica_oznaka"],
		Opstina:                         podesavanja["opstina"],
		Grad:                            podesavanja["grad"],
		LogoPutanja:                     podesavanja["logo_putanja"],
		TopbarLogoSlika:                 podesavanja["topbar_logo_slika"] == "1",
		FirmaPravniOblik:                vrednostIliDefault(podesavanja, "firma_pravni_oblik", "pausalac"),
		FirmaPdvObveznik:                vrednostIliDefault(podesavanja, "firma_pdv_obveznik", "ne"),
		FirmaFiskalizacija:              vrednostIliDefault(podesavanja, "firma_fiskalizacija", "ne"),
		FirmaRezim:                      vrednostIliDefault(podesavanja, "firma_rezim", "samo_evidencija"),
		Sacuvano:                        r.URL.Query().Get("sacuvano") == "1",
		BackupVracen:                    r.URL.Query().Get("sacuvano") == "vraceno",
		Verzija:                         h.Verzija,
		LogoGreska:                      r.URL.Query().Get("logo_greska"),
		Backupi:                         ucitajListuBackupa(),
		LoginPozadina:                   podesavanja["login_pozadina"],
		LoginPozadinaOpacity:            vrednostIliDefault(podesavanja, "login_pozadina_opacity", "50"),
		LoginPozadinaBlurPozadine:       vrednostIliDefault(podesavanja, "login_pozadina_blur_pozadine", "0"),
		LoginPozadinaBlurKartice:        vrednostIliDefault(podesavanja, "login_pozadina_blur_kartice", "12"),
		LoginPozadinaZatamnjenjeKartice: vrednostIliDefault(podesavanja, "login_pozadina_zatamnjenje_kartice", "0"),
		BackupIntervalSati:              vrednostIliDefault(podesavanja, "backup_interval_sati", "24"),
		BackupBrojKopija:                vrednostIliDefault(podesavanja, "backup_broj_kopija", "7"),
		KalkulacijaMarza:                vrednostIliDefault(podesavanja, "kalkulacija_marza", "20"),
		ServisGarancijaDana:             vrednostIliDefault(podesavanja, "servis_garancija_dana", "60"),
		ServisCenaDijagnostike:          vrednostIliDefault(podesavanja, "servis_cena_dijagnostike", "0"),
		PredracunRokDana:                vrednostIliDefault(podesavanja, "predracun_rok_dana", "7"),
		PredvidjenRokDana:               vrednostIliDefault(podesavanja, "predvidjen_rok_dana", "15"),
		ServisUslovi:                    vrednostIliDefault(podesavanja, "servis_uslovi", podrazumevaniUsloviServisa),
		ServisKlauzulaPredracuna:        vrednostIliDefault(podesavanja, "servis_klauzula_predracuna", podrazumevanaKlauzulaPredracuna),
		QrBazniUrl:                      podesavanja["qr_bazni_url"],
		PfrUrl:                          vrednostIliDefault(podesavanja, "pfr_url", "http://127.0.0.1:4566"),
		PfrTip:                          vrednostIliDefault(podesavanja, "pfr_tip", "teron"),
		PfrKasir:                        podesavanja["pfr_kasir"],
		PfrApiKey:                       podesavanja["pfr_api_key"],
		VerifyHost:                      podesavanja["verify_host"],
		BePin:                           vrednostIliDefault(podesavanja, "be_pin", "1234"),
		BeLimit:                         vrednostIliDefault(podesavanja, "be_limit", "500000"),
		BeEnabled:                       vrednostIliDefault(podesavanja, "be_enabled", "true"),
		FiskalPismo:                     vrednostIliDefault(podesavanja, "fiskalni_pismo", "latin"),
	}, nil
}

// PodesavanjaOpste renderuje stranicu sa opštim podešavanjima (firma i logo)
func (h *Handler) PodesavanjaOpste(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "podesavanja.pregled"); !ok {
		return
	}
	podaci, err := h.napuniPodaciPodesavanja(r, "Podešavanja — Opšte")
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}
	podaci.Stranica = "podesavanja-opste"
	h.renderujTemplate(w, "podesavanja_opste", podaci)
}

// PodesavanjaIzgled renderuje stranicu sa podešavanjima izgleda (pozadine i tema)
func (h *Handler) PodesavanjaIzgled(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "podesavanja.pregled"); !ok {
		return
	}
	podaci, err := h.napuniPodaciPodesavanja(r, "Podešavanja — Izgled")
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}
	podaci.Stranica = "podesavanja-izgled"
	h.renderujTemplate(w, "podesavanja_izgled", podaci)
}

// PodesavanjaFiskalizacija renderuje stranicu sa podešavanjima L-PFR veze
func (h *Handler) PodesavanjaFiskalizacija(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "podesavanja.pregled"); !ok {
		return
	}
	podaci, err := h.napuniPodaciPodesavanja(r, "Podešavanja — Fiskalizacija")
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}
	podaci.Stranica = "podesavanja-fiskalizacija"
	h.renderujTemplate(w, "podesavanja_fiskalizacija", podaci)
}

// TestFiskalizacije udara /api/status na konfigurisanom PFR URL-u i vraća HTMX fragment
func (h *Handler) TestFiskalizacije(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "podesavanja.pregled"); !ok {
		return
	}
	pfrURL := strings.TrimSpace(r.FormValue("pfr_url"))
	if pfrURL == "" {
		podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		if err != nil {
			http.Error(w, "Greška", http.StatusInternalServerError)
			return
		}
		pfrURL = vrednostIliDefault(podesavanja, "pfr_url", "http://127.0.0.1:4566")
	}

	// SSRF zaštita: dozvoljeni su samo localhost i privatne mreže (192.168.x.x,
	// 10.x.x.x, 172.16-31.x.x, 127.x.x.x). URL rekonstruišemo kroz url.URL{}
	// iz validiranih komponenti da CodeQL prepozna sanitizaciju.
	parsedURL, err := url.Parse(pfrURL)
	if err != nil {
		http.Error(w, "Nevažeći PFR URL", http.StatusBadRequest)
		return
	}
	host := parsedURL.Hostname()
	if !jePrivatnaAdresa(host) {
		http.Error(w, "Nevažeći PFR URL — dozvoljeni su samo lokalni/privatni hostovi (127.x, 192.168.x, 10.x, 172.16-31.x)", http.StatusBadRequest)
		return
	}
	port := 4566
	if p := parsedURL.Port(); p != "" {
		if n, e := strconv.Atoi(p); e == nil && n > 0 && n <= 65535 {
			port = n
		}
	}
	safeURL := url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(host, strconv.Itoa(port)),
		Path:   "/api/status",
	}
	statusURL := safeURL.String()

	klijent := &http.Client{Timeout: 5 * time.Second}
	resp, err := klijent.Get(statusURL) // lgtm[go/request-forgery] -- host je validiran kroz jePrivatnaAdresa (privatne/localhost adrese)
	if err != nil {
		slog.Error("test fiskalizacije: konekcija nije uspela", "url", statusURL, "error", err)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<div class="fisk-status greska">&#10007; Nije dostupan — proverite adresu i mrežu</div>`)
		return
	}
	defer resp.Body.Close()

	var status map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		slog.Error("test fiskalizacije: neispravan odgovor servera", "url", statusURL, "error", err)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<div class="fisk-status greska">&#10007; Neispravan odgovor servera</div>`)
		return
	}

	sdcDateTime, _ := status["sdcDateTime"].(string)
	lastInvoice, _ := status["lastInvoiceNumber"].(string)
	tin, _ := status["tin"].(string)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<div class="fisk-status uspeh">
		&#10003; Povezano — %s
		<div class="fisk-status-detalji">
			<span>TIN: %s</span>
			<span>Poslednji račun: %s</span>
			<span>Vreme: %s</span>
		</div>
	</div>`, html.EscapeString(pfrURL), html.EscapeString(tin), html.EscapeString(lastInvoice), html.EscapeString(sdcDateTime))
}

// BeStatus se spaja na lokalni kartica emulator (TCP :4567) i vraća HTMX fragment sa statusom.
func (h *Handler) BeStatus(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "podesavanja.pregled"); !ok {
		return
	}

	bePort := os.Getenv("BE_PORT")
	if bePort == "" {
		bePort = "4567"
	}
	addr := "127.0.0.1:" + bePort

	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		slog.Error("BeStatus: emulator nedostupan", "addr", addr, "error", err)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<div class="fisk-status greska">&#10007; Emulator nije dostupan na %s</div>`,
			html.EscapeString(addr))
		return
	}
	defer conn.Close()

	cmd := `{"command":"status"}` + "\n"
	conn.Write([]byte(cmd))
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 4096)
	n, _ := conn.Read(buf)
	conn.Close()

	var st map[string]any
	if err := json.Unmarshal(buf[:n], &st); err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<div class="fisk-status greska">&#10007; Neispravan odgovor</div>`)
		return
	}

	total, _ := st["total_counter"].(float64)
	unread, _ := st["unread_amount"].(float64)
	limit, _ := st["limit"].(float64)
	pinReq, _ := st["pin_required"].(bool)

	pinTekst := "PIN unesen"
	if pinReq {
		pinTekst = "Čeka PIN"
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<div class="fisk-status uspeh">
		&#10003; Kartica emulator aktivan
		<div class="fisk-status-detalji">
			<span>Ukupno računa: %d</span>
			<span>Neisčitano: %s / %s din</span>
			<span>%s</span>
		</div>
	</div>`,
		int(total),
		html.EscapeString(formatirajDinare(unread, 0)),
		html.EscapeString(formatirajDinare(limit, 0)),
		html.EscapeString(pinTekst),
	)
}

// BeResetAudit šalje reset_audit komandu kartica emulatoru i vraća HTMX fragment.
func (h *Handler) BeResetAudit(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "podesavanja.izmeni"); !ok {
		return
	}
	bePort := os.Getenv("BE_PORT")
	if bePort == "" {
		bePort = "4567"
	}
	conn, err := net.DialTimeout("tcp", "127.0.0.1:"+bePort, 2*time.Second)
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<div class="fisk-status greska">&#10007; Emulator nije dostupan</div>`)
		return
	}
	conn.Write([]byte(`{"command":"reset_audit"}` + "\n"))
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 512)
	n, _ := conn.Read(buf)
	conn.Close()

	var resp map[string]any
	json.Unmarshal(buf[:n], &resp)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<div class="fisk-status uspeh">&#10003; Neisčitani iznos resetovan na 0 (Proof of Audit simuliran)</div>`)
}

// jePrivatnaAdresa proverava da li je hostname u opsegu privatnih/lokalnih mreža.
// Dozvoljeni: 127.x.x.x, localhost, 10.x.x.x, 172.16-31.x.x, 192.168.x.x.
func jePrivatnaAdresa(hostname string) bool {
	if hostname == "localhost" {
		return true
	}
	if ip := net.ParseIP(hostname); ip != nil {
		return jePrivatnaIP(ip)
	}
	// Hostname (npr. Docker Compose service name kao "teron-mock") — razreši
	// preko DNS-a i dozvoli samo ako SVE dobijene adrese spadaju u privatne
	// opsege (localhost/127/10/172.16-31/192.168).
	ips, err := net.LookupIP(hostname)
	if err != nil || len(ips) == 0 {
		return false
	}
	for _, ip := range ips {
		if !jePrivatnaIP(ip) {
			return false
		}
	}
	return true
}

func jePrivatnaIP(ip net.IP) bool {
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}
	return ip4[0] == 127 || // loopback
		ip4[0] == 10 || // privatna klasa A
		(ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31) || // privatna klasa B
		(ip4[0] == 192 && ip4[1] == 168) // privatna klasa C
}

// PodesavanjaSistem renderuje stranicu sa sistemskim podešavanjima (backup)
func (h *Handler) PodesavanjaSistem(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "podesavanja.pregled"); !ok {
		return
	}
	podaci, err := h.napuniPodaciPodesavanja(r, "Podešavanja — Sistem")
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}
	podaci.Stranica = "podesavanja-sistem"
	h.renderujTemplate(w, "podesavanja_sistem", podaci)
}
