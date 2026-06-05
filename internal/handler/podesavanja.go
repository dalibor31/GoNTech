package handler

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	ntechsqlite "ntech/internal/db/sqlite"
	"ntech/internal/middleware"
	"ntech/internal/model"

	"github.com/go-chi/chi/v5"
)

// PodaciPodesavanja su podaci za stranicu podešavanja
type PodaciPodesavanja struct {
	model.PodaciStranice
	NazivFirme  string
	Podnazlov   string
	Adresa      string
	Telefon     string
	PIB         string
	LogoTip     string
	LogoPutanja string
	Tema        string
	Sacuvano    bool
	Verzija     string
	LogoGreska   string
	BackupVracen bool
	Backupi      []BackupInfo
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
	k := middleware.KorisnikIzKonteksta(r.Context())
	if k == nil || !h.DozvoleRepo.ImaDozvolu(r.Context(), k.Uloga, "podesavanja.pregled") {
		http.Error(w, "Nemate dozvolu za ovu akciju.", http.StatusForbidden)
		return
	}
	podesavanja, err := ntechsqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "podesavanja"
	ps.NaslovStranice = "Podešavanja"
	podaci := PodaciPodesavanja{
		PodaciStranice: ps,
		NazivFirme:     podesavanja["naziv_firme"],
		Podnazlov:      podesavanja["podnazlov"],
		Adresa:         podesavanja["adresa"],
		Telefon:        podesavanja["telefon"],
		PIB:            podesavanja["pib"],
		LogoTip:        podesavanja["logo_tip"],
		LogoPutanja:    podesavanja["logo_putanja"],
		Tema:           podesavanja["tema"],
		Sacuvano:       r.URL.Query().Get("sacuvano") == "1",
		BackupVracen:   r.URL.Query().Get("sacuvano") == "vraceno",
		Verzija:        h.Verzija,
		LogoGreska:     r.URL.Query().Get("logo_greska"),
		Backupi:        ucitajListuBackupa(),
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

// VratiBackup zamenjuje trenutnu bazu sa izabranim backup fajlom
func (h *Handler) VratiBackup(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if k == nil || !h.DozvoleRepo.ImaDozvolu(r.Context(), k.Uloga, "backup.pokreni") {
		http.Error(w, "Nemate dozvolu za ovu akciju.", http.StatusForbidden)
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
		log.Printf("vrati backup: greška pri kreiranju sigurnosne kopije: %v", err)
		http.Redirect(w, r, "/podesavanja?backup_greska=Greška+pri+kreiranju+sigurnosne+kopije", http.StatusSeeOther)
		return
	}

	// isprazni WAL u glavni fajl
	if _, err := h.DB.ExecContext(r.Context(), "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		log.Printf("vrati backup: wal_checkpoint greška: %v", err)
	}

	// zatvori sve konekcije
	if err := h.DB.Close(); err != nil {
		log.Printf("vrati backup: greška pri zatvaranju baze: %v", err)
	}

	// kopiraj backup fajl na mesto trenutne baze
	if err := kopiraFajl(putanjaBackupa, h.PutanjaBaze); err != nil {
		log.Printf("vrati backup: greška pri kopiranju: %v", err)
		http.Redirect(w, r, "/podesavanja?backup_greska=Greška+pri+obnovi+baze", http.StatusSeeOther)
		return
	}

	// ukloni WAL i SHM fajlove stare baze
	os.Remove(h.PutanjaBaze + "-wal")
	os.Remove(h.PutanjaBaze + "-shm")

	// otvori novu konekciju
	novaDB, err := ntechsqlite.OtvoriDB(h.PutanjaBaze)
	if err != nil {
		log.Printf("vrati backup: greška pri otvaranju nove baze: %v", err)
		http.Redirect(w, r, "/podesavanja?backup_greska=Baza+obnovljena+ali+je+potreban+restart", http.StatusSeeOther)
		return
	}

	h.reinicijalzijRepozitorijume(novaDB)
	log.Printf("Baza uspešno obnovljena iz: %s", ime)

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

// SacuvajPodesavanja prima POST i čuva podešavanja u bazu
func (h *Handler) SacuvajPodesavanja(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if k == nil || !h.DozvoleRepo.ImaDozvolu(r.Context(), k.Uloga, "podesavanja.izmeni") {
		http.Error(w, "Nemate dozvolu za ovu akciju.", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	polja := map[string]string{
		"naziv_firme": r.FormValue("naziv_firme"),
		"podnazlov":   r.FormValue("podnazlov"),
		"adresa":      r.FormValue("adresa"),
		"telefon":     r.FormValue("telefon"),
		"pib":         r.FormValue("pib"),
		"logo_tip":    r.FormValue("logo_tip"),
		"tema":        r.FormValue("tema"),
	}

	for kljuc, vrednost := range polja {
		if vrednost == "" {
			continue
		}
		if err := ntechsqlite.SacuvajPodesavanje(r.Context(), h.DB, kljuc, vrednost); err != nil {
			http.Error(w, "Greška pri čuvanju podešavanja", http.StatusInternalServerError)
			return
		}
	}

	http.Redirect(w, r, "/podesavanja?sacuvano=1", http.StatusSeeOther)
}

// BackupBaze kreira konzistentnu kopiju baze i šalje je kao attachment
func (h *Handler) BackupBaze(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if k == nil || !h.DozvoleRepo.ImaDozvolu(r.Context(), k.Uloga, "backup.pregled") {
		http.Error(w, "Nemate dozvolu za ovu akciju.", http.StatusForbidden)
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
	k := middleware.KorisnikIzKonteksta(r.Context())
	if k == nil || !h.DozvoleRepo.ImaDozvolu(r.Context(), k.Uloga, "podesavanja.izmeni") {
		http.Error(w, "Nemate dozvolu za ovu akciju.", http.StatusForbidden)
		return
	}
	// ograničavamo telo zahteva na 2MB + malo za zaglavlja forme
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20+4096)
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		http.Redirect(w, r, "/podesavanja?logo_greska=Fajl+je+prevelik+%28maksimum+2+MB%29", http.StatusSeeOther)
		return
	}

	fajl, zaglavlje, err := r.FormFile("logo")
	if err != nil {
		http.Redirect(w, r, "/podesavanja?logo_greska=Nije+odabran+fajl", http.StatusSeeOther)
		return
	}
	defer fajl.Close()

	// eksplicitna provera veličine (zaglavlje.Size je postavljeno od strane browsera)
	if zaglavlje.Size > 2<<20 {
		http.Redirect(w, r, "/podesavanja?logo_greska=Fajl+je+prevelik+%28maksimum+2+MB%29", http.StatusSeeOther)
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
		http.Redirect(w, r, "/podesavanja?logo_greska=Dozvoljeni+formati+su+PNG%2C+JPG+i+SVG", http.StatusSeeOther)
		return
	}

	// za binarne formate proveravamo i stvarni tip fajla (SVG je tekstualni, preskačemo)
	if ext != ".svg" {
		buf := make([]byte, 512)
		n, _ := fajl.Read(buf)
		stvarniMime := http.DetectContentType(buf[:n])
		if !strings.HasPrefix(stvarniMime, ocekivaniMime) {
			http.Redirect(w, r, "/podesavanja?logo_greska=Sadržaj+fajla+ne+odgovara+odabranoj+ekstenziji", http.StatusSeeOther)
			return
		}
		// vraćamo kursor na početak
		if _, err := fajl.Seek(0, io.SeekStart); err != nil {
			http.Redirect(w, r, "/podesavanja?logo_greska=Greška+pri+obradi+fajla", http.StatusSeeOther)
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
		log.Printf("upload loga: ne mogu kreirati fajl: %v", err)
		http.Redirect(w, r, "/podesavanja?logo_greska=Greška+pri+čuvanju+fajla", http.StatusSeeOther)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, fajl); err != nil {
		log.Printf("upload loga: greška pri kopiranju: %v", err)
		http.Redirect(w, r, "/podesavanja?logo_greska=Greška+pri+čuvanju+fajla", http.StatusSeeOther)
		return
	}

	// timestamp u URL-u sprečava browser da koristi staru keširanu sliku
	putanja := fmt.Sprintf("/static/uploads/logo%s?v=%d", ext, time.Now().Unix())
	if err := ntechsqlite.SacuvajPodesavanje(r.Context(), h.DB, "logo_putanja", putanja); err != nil {
		log.Printf("upload loga: greška pri čuvanju putanje: %v", err)
		http.Redirect(w, r, "/podesavanja?logo_greska=Greška+pri+čuvanju+podešavanja", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/podesavanja?sacuvano=1", http.StatusSeeOther)
}

// PromeniTemu menja aktivnu temu i vraća na prethodnu stranicu
func (h *Handler) PromeniTemu(w http.ResponseWriter, r *http.Request) {
	tema := chi.URLParam(r, "tema")

	// proveravamo da li je validna tema
	validne := map[string]bool{
		"tamna": true, "svetla": true,
	}
	if !validne[tema] {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}

	if err := ntechsqlite.SacuvajPodesavanje(r.Context(), h.DB, "tema", tema); err != nil {
		http.Error(w, "Greška pri promeni teme", http.StatusInternalServerError)
		return
	}

	// vraćamo se na stranicu sa koje je kliknut dugmić
	referer := r.Header.Get("Referer")
	if referer == "" {
		referer = "/dashboard"
	}
	http.Redirect(w, r, referer, http.StatusSeeOther)
}
