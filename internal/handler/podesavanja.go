package handler

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
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
	NazivFirme      string
	Podnazlov       string
	Adresa          string
	Telefon         string
	PIB             string
	LogoTip         string
	LogoPutanja     string
	GlobalnaTema    string // stvarna vrednost iz podešavanja (ne senči PodaciStranice.Tema)
	Sacuvano        bool
	Verzija         string
	LogoGreska      string
	BackupVracen    bool
	Backupi         []BackupInfo
	LoginPozadina             string
	LoginPozadinaOpacity      string
	LoginPozadinaBlurPozadine string
	LoginPozadinaBlurKartice  string
	AppPozadina               string
	AppPozadinaOpacity        string
	AppPozadinaBlur           string
	AppPozadinaBlurPozadine   string
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
		GlobalnaTema: func() string {
			if v := podesavanja["globalna_tema"]; v != "" {
				return v
			}
			if v := podesavanja["tema"]; v != "" {
				return v
			}
			return "tamna"
		}(),
		Sacuvano:       r.URL.Query().Get("sacuvano") == "1",
		BackupVracen:   r.URL.Query().Get("sacuvano") == "vraceno",
		Verzija:        h.Verzija,
		LogoGreska:     r.URL.Query().Get("logo_greska"),
		Backupi:        ucitajListuBackupa(),
		LoginPozadina: podesavanja["login_pozadina"],
		LoginPozadinaOpacity: func() string {
			v := podesavanja["login_pozadina_opacity"]
			if v == "" { return "50" }
			return v
		}(),
		LoginPozadinaBlurPozadine: func() string {
			v := podesavanja["login_pozadina_blur_pozadine"]
			if v == "" { return "0" }
			return v
		}(),
		LoginPozadinaBlurKartice: func() string {
			v := podesavanja["login_pozadina_blur_kartice"]
			if v == "" { return "12" }
			return v
		}(),
		AppPozadina: podesavanja["app_pozadina"],
		AppPozadinaOpacity: func() string {
			v := podesavanja["app_pozadina_opacity"]
			if v == "" { return "50" }
			return v
		}(),
		AppPozadinaBlur: func() string {
			v := podesavanja["app_pozadina_blur"]
			if v == "" { return "12" }
			return v
		}(),
		AppPozadinaBlurPozadine: func() string {
			v := podesavanja["app_pozadina_blur_pozadine"]
			if v == "" { return "0" }
			return v
		}(),
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
		"naziv_firme":  r.FormValue("naziv_firme"),
		"podnazlov":    r.FormValue("podnazlov"),
		"adresa":       r.FormValue("adresa"),
		"telefon":      r.FormValue("telefon"),
		"pib":          r.FormValue("pib"),
		"logo_tip":     r.FormValue("logo_tip"),
		"tema":         r.FormValue("tema"),
		"globalna_tema": r.FormValue("globalna_tema"),
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

	sledeci := r.FormValue("_next")
	if sledeci == "" {
		http.Redirect(w, r, "/podesavanja?sacuvano=1", http.StatusSeeOther)
	} else {
		http.Redirect(w, r, sledeci+"?sacuvano=1", http.StatusSeeOther)
	}
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
	k := middleware.KorisnikIzKonteksta(r.Context())
	if k == nil || !h.DozvoleRepo.ImaDozvolu(r.Context(), k.Uloga, "podesavanja.login_pozadina") {
		http.Error(w, "Nemate dozvolu za ovu akciju.", http.StatusForbidden)
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
	staraPodesavanja, _ := ntechsqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if stara := staraPodesavanja["login_pozadina"]; stara != "" {
		// putanja u bazi je oblika /static/uploads/ime.ext?v=..., izvlačimo samo ime fajla
		deoBezverzije := strings.Split(stara, "?")[0]
		staroIme := filepath.Base(deoBezverzije)
		os.Remove(filepath.Join("web/static/uploads", staroIme))
	}

	novoIme, err := generisiImeUploada(ext)
	if err != nil {
		log.Printf("upload login pozadine: greška pri generisanju imena: %v", err)
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju fajla.")
		http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
		return
	}

	odrediste := filepath.Join("web/static/uploads", novoIme)
	dst, err := os.Create(odrediste)
	if err != nil {
		log.Printf("upload login pozadine: ne mogu kreirati fajl: %v", err)
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju fajla.")
		http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, fajl); err != nil {
		log.Printf("upload login pozadine: greška pri kopiranju: %v", err)
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju fajla.")
		http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
		return
	}

	putanja := fmt.Sprintf("/static/uploads/%s?v=%d", novoIme, time.Now().Unix())
	if err := ntechsqlite.SacuvajPodesavanje(r.Context(), h.DB, "login_pozadina", putanja); err != nil {
		log.Printf("upload login pozadine: greška pri čuvanju putanje: %v", err)
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju podešavanja.")
		http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
		return
	}

	middleware.SetFlash(w, r, h.DB, "uspeh", "Pozadinska slika je uspešno otpremljena.")
	http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
}

// UkloniLoginPozadinu briše pozadinsku sliku login stranice sa diska i iz podešavanja
func (h *Handler) UkloniLoginPozadinu(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if k == nil || !h.DozvoleRepo.ImaDozvolu(r.Context(), k.Uloga, "podesavanja.login_pozadina") {
		http.Error(w, "Nemate dozvolu za ovu akciju.", http.StatusForbidden)
		return
	}

	podesavanja, err := ntechsqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err == nil {
		if stara := podesavanja["login_pozadina"]; stara != "" {
			deoBezverzije := strings.Split(stara, "?")[0]
			staroIme := filepath.Base(deoBezverzije)
			os.Remove(filepath.Join("web/static/uploads", staroIme))
		}
	}

	if err := ntechsqlite.SacuvajPodesavanje(r.Context(), h.DB, "login_pozadina", ""); err != nil {
		log.Printf("ukloni login pozadinu: greška pri čuvanju: %v", err)
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri uklanjanju slike.")
		http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
		return
	}

	middleware.SetFlash(w, r, h.DB, "uspeh", "Pozadinska slika je uklonjena.")
	http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
}

// OtpremiAppPozadinu prima multipart upload slike i čuva je kao pozadinsku sliku aplikacije
func (h *Handler) OtpremiAppPozadinu(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if k == nil || !h.DozvoleRepo.ImaDozvolu(r.Context(), k.Uloga, "podesavanja.app_pozadina") {
		http.Error(w, "Nemate dozvolu za ovu akciju.", http.StatusForbidden)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 5<<20+4096)
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Fajl je prevelik (maksimum 5 MB).")
		http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
		return
	}

	fajl, zaglavlje, err := r.FormFile("app_pozadina")
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

	// briše staru app pozadinu sa diska ako postoji
	staraPodesavanja, _ := ntechsqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if stara := staraPodesavanja["app_pozadina"]; stara != "" {
		deoBezverzije := strings.Split(stara, "?")[0]
		staroIme := filepath.Base(deoBezverzije)
		os.Remove(filepath.Join("web/static/uploads", staroIme))
	}

	novoIme, err := generisiImeUploada(ext)
	if err != nil {
		log.Printf("upload app pozadine: greška pri generisanju imena: %v", err)
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju fajla.")
		http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
		return
	}

	odrediste := filepath.Join("web/static/uploads", novoIme)
	dst, err := os.Create(odrediste)
	if err != nil {
		log.Printf("upload app pozadine: ne mogu kreirati fajl: %v", err)
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju fajla.")
		http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, fajl); err != nil {
		log.Printf("upload app pozadine: greška pri kopiranju: %v", err)
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju fajla.")
		http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
		return
	}

	putanja := fmt.Sprintf("/static/uploads/%s?v=%d", novoIme, time.Now().Unix())
	if err := ntechsqlite.SacuvajPodesavanje(r.Context(), h.DB, "app_pozadina", putanja); err != nil {
		log.Printf("upload app pozadine: greška pri čuvanju putanje: %v", err)
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju podešavanja.")
		http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
		return
	}

	// zapamti trenutnu globalnu temu pre nego što je forsiramo na tamnu
	trenutnaTema := staraPodesavanja["globalna_tema"]
	if trenutnaTema == "" {
		trenutnaTema = staraPodesavanja["tema"]
	}
	if trenutnaTema == "" {
		trenutnaTema = "tamna"
	}
	_ = ntechsqlite.SacuvajPodesavanje(r.Context(), h.DB, "tema_pre_slike", trenutnaTema)
	_ = ntechsqlite.SacuvajPodesavanje(r.Context(), h.DB, "globalna_tema", "tamna")

	middleware.SetFlash(w, r, h.DB, "uspeh", "Pozadinska slika aplikacije je uspešno otpremljena.")
	http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
}

// UkloniAppPozadinu briše pozadinsku sliku aplikacije sa diska i iz podešavanja
func (h *Handler) UkloniAppPozadinu(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if k == nil || !h.DozvoleRepo.ImaDozvolu(r.Context(), k.Uloga, "podesavanja.app_pozadina") {
		http.Error(w, "Nemate dozvolu za ovu akciju.", http.StatusForbidden)
		return
	}

	podesavanja, err := ntechsqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err == nil {
		if stara := podesavanja["app_pozadina"]; stara != "" {
			deoBezverzije := strings.Split(stara, "?")[0]
			staroIme := filepath.Base(deoBezverzije)
			os.Remove(filepath.Join("web/static/uploads", staroIme))
		}
	}

	if err := ntechsqlite.SacuvajPodesavanje(r.Context(), h.DB, "app_pozadina", ""); err != nil {
		log.Printf("ukloni app pozadinu: greška pri čuvanju: %v", err)
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri uklanjanju slike.")
		http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
		return
	}

	// vrati temu koja je bila aktivna pre dodavanja slike
	if err == nil {
		temaPreSlike := podesavanja["tema_pre_slike"]
		if temaPreSlike == "" {
			temaPreSlike = "tamna"
		}
		_ = ntechsqlite.SacuvajPodesavanje(r.Context(), h.DB, "globalna_tema", temaPreSlike)
		_ = ntechsqlite.SacuvajPodesavanje(r.Context(), h.DB, "tema_pre_slike", "")
	}

	middleware.SetFlash(w, r, h.DB, "uspeh", "Pozadinska slika aplikacije je uklonjena.")
	http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
}

// SacuvajAppPozadinaStilove čuva vrednosti zamućenja i prozirnosti pozadine aplikacije
func (h *Handler) SacuvajAppPozadinaStilove(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if k == nil || !h.DozvoleRepo.ImaDozvolu(r.Context(), k.Uloga, "podesavanja.app_pozadina") {
		http.Error(w, "Nemate dozvolu za ovu akciju.", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čitanju forme.")
		http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
		return
	}

	blurStr := r.FormValue("blur")
	blurPozadineStr := r.FormValue("blur_pozadine")
	opacityStr := r.FormValue("opacity")

	blurVal, err := strconv.Atoi(blurStr)
	if err != nil || blurVal < 0 || blurVal > 20 {
		middleware.SetFlash(w, r, h.DB, "greska", "Neispravna vrednost zamućenja stakla.")
		http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
		return
	}
	blurPozadineVal, err := strconv.Atoi(blurPozadineStr)
	if err != nil || blurPozadineVal < 0 || blurPozadineVal > 20 {
		middleware.SetFlash(w, r, h.DB, "greska", "Neispravna vrednost zamućenja pozadine.")
		http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
		return
	}
	opacityVal, err := strconv.Atoi(opacityStr)
	if err != nil || opacityVal < 0 || opacityVal > 80 {
		middleware.SetFlash(w, r, h.DB, "greska", "Neispravna vrednost prozirnosti.")
		http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
		return
	}

	for kljuc, vrednost := range map[string]string{
		"app_pozadina_blur":          blurStr,
		"app_pozadina_blur_pozadine": blurPozadineStr,
		"app_pozadina_opacity":       opacityStr,
	} {
		if err := ntechsqlite.SacuvajPodesavanje(r.Context(), h.DB, kljuc, vrednost); err != nil {
			log.Printf("stilovi app pozadine: greška pri čuvanju %s: %v", kljuc, err)
			middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju podešavanja.")
			http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
			return
		}
	}

	middleware.SetFlash(w, r, h.DB, "uspeh", "Izgled pozadine aplikacije je sačuvan.")
	http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
}

// SacuvajLoginPozadinaStilove čuva vrednosti zamućenja i prozirnosti pozadine login stranice
func (h *Handler) SacuvajLoginPozadinaStilove(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if k == nil || !h.DozvoleRepo.ImaDozvolu(r.Context(), k.Uloga, "podesavanja.login_pozadina") {
		http.Error(w, "Nemate dozvolu za ovu akciju.", http.StatusForbidden)
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

	for kljuc, vrednost := range map[string]string{
		"login_pozadina_blur_pozadine": blurPozadineStr,
		"login_pozadina_blur_kartice":  blurKarticeStr,
		"login_pozadina_opacity":       opacityStr,
	} {
		if err := ntechsqlite.SacuvajPodesavanje(r.Context(), h.DB, kljuc, vrednost); err != nil {
			log.Printf("stilovi login pozadine: greška pri čuvanju %s: %v", kljuc, err)
			middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju podešavanja.")
			http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
			return
		}
	}

	middleware.SetFlash(w, r, h.DB, "uspeh", "Izgled pozadine prijave je sačuvan.")
	http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
}

// napuniPodaciPodesavanja učitava sva podešavanja i kreira strukturu za template
func (h *Handler) napuniPodaciPodesavanja(r *http.Request, naslov string) (PodaciPodesavanja, error) {
	podesavanja, err := ntechsqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		return PodaciPodesavanja{}, err
	}
	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "podesavanja"
	ps.NaslovStranice = naslov
	return PodaciPodesavanja{
		PodaciStranice: ps,
		NazivFirme:     podesavanja["naziv_firme"],
		Podnazlov:      podesavanja["podnazlov"],
		Adresa:         podesavanja["adresa"],
		Telefon:        podesavanja["telefon"],
		PIB:            podesavanja["pib"],
		LogoTip:        podesavanja["logo_tip"],
		LogoPutanja:    podesavanja["logo_putanja"],
		GlobalnaTema: func() string {
			if v := podesavanja["globalna_tema"]; v != "" {
				return v
			}
			if v := podesavanja["tema"]; v != "" {
				return v
			}
			return "tamna"
		}(),
		Sacuvano:       r.URL.Query().Get("sacuvano") == "1",
		BackupVracen:   r.URL.Query().Get("sacuvano") == "vraceno",
		Verzija:        h.Verzija,
		LogoGreska:     r.URL.Query().Get("logo_greska"),
		Backupi:        ucitajListuBackupa(),
		LoginPozadina:  podesavanja["login_pozadina"],
		LoginPozadinaOpacity: func() string {
			if v := podesavanja["login_pozadina_opacity"]; v != "" {
				return v
			}
			return "50"
		}(),
		LoginPozadinaBlurPozadine: func() string {
			if v := podesavanja["login_pozadina_blur_pozadine"]; v != "" {
				return v
			}
			return "0"
		}(),
		LoginPozadinaBlurKartice: func() string {
			if v := podesavanja["login_pozadina_blur_kartice"]; v != "" {
				return v
			}
			return "12"
		}(),
		AppPozadina: podesavanja["app_pozadina"],
		AppPozadinaOpacity: func() string {
			if v := podesavanja["app_pozadina_opacity"]; v != "" {
				return v
			}
			return "50"
		}(),
		AppPozadinaBlur: func() string {
			if v := podesavanja["app_pozadina_blur"]; v != "" {
				return v
			}
			return "12"
		}(),
		AppPozadinaBlurPozadine: func() string {
			if v := podesavanja["app_pozadina_blur_pozadine"]; v != "" {
				return v
			}
			return "0"
		}(),
	}, nil
}

// PodesavanjaOpste renderuje stranicu sa opštim podešavanjima (firma i logo)
func (h *Handler) PodesavanjaOpste(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if k == nil || !h.DozvoleRepo.ImaDozvolu(r.Context(), k.Uloga, "podesavanja.pregled") {
		http.Error(w, "Nemate dozvolu za ovu akciju.", http.StatusForbidden)
		return
	}
	podaci, err := h.napuniPodaciPodesavanja(r, "Podešavanja — Opšte")
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}
	h.renderujTemplate(w, "podesavanja_opste", podaci)
}

// PodesavanjaIzgled renderuje stranicu sa podešavanjima izgleda (pozadine i tema)
func (h *Handler) PodesavanjaIzgled(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if k == nil || !h.DozvoleRepo.ImaDozvolu(r.Context(), k.Uloga, "podesavanja.pregled") {
		http.Error(w, "Nemate dozvolu za ovu akciju.", http.StatusForbidden)
		return
	}
	podaci, err := h.napuniPodaciPodesavanja(r, "Podešavanja — Izgled")
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}
	h.renderujTemplate(w, "podesavanja_izgled", podaci)
}

// PodesavanjaSistem renderuje stranicu sa sistemskim podešavanjima (backup)
func (h *Handler) PodesavanjaSistem(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if k == nil || !h.DozvoleRepo.ImaDozvolu(r.Context(), k.Uloga, "podesavanja.pregled") {
		http.Error(w, "Nemate dozvolu za ovu akciju.", http.StatusForbidden)
		return
	}
	podaci, err := h.napuniPodaciPodesavanja(r, "Podešavanja — Sistem")
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}
	h.renderujTemplate(w, "podesavanja_sistem", podaci)
}

// PromeniTemu menja aktivnu temu i vraća na prethodnu stranicu (stara GET ruta, zadržana za kompatibilnost)
func (h *Handler) PromeniTemu(w http.ResponseWriter, r *http.Request) {
	tema := chi.URLParam(r, "tema")

	validne := map[string]bool{"tamna": true, "svetla": true}
	if !validne[tema] {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}

	_ = ntechsqlite.SacuvajPodesavanje(r.Context(), h.DB, "globalna_tema", tema)

	referer := r.Header.Get("Referer")
	if referer == "" {
		referer = "/dashboard"
	}
	http.Redirect(w, r, referer, http.StatusSeeOther)
}

// PromeniGlobalnuTemu prima POST sa poljem "tema" i čuva u globalna_tema u podešavanjima
func (h *Handler) PromeniGlobalnuTemu(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	tema := r.FormValue("tema")
	validne := map[string]bool{"tamna": true, "svetla": true}
	if !validne[tema] {
		tema = "tamna"
	}

	if err := ntechsqlite.SacuvajPodesavanje(r.Context(), h.DB, "globalna_tema", tema); err != nil {
		http.Error(w, "Greška pri promeni teme", http.StatusInternalServerError)
		return
	}

	referer := r.Header.Get("Referer")
	if referer == "" {
		referer = "/dashboard"
	}
	http.Redirect(w, r, referer, http.StatusSeeOther)
}

// PrimeniGlobalnuTemu čuva globalnu temu i resetuje lokalne teme svih korisnika
func (h *Handler) PrimeniGlobalnuTemu(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if !h.DozvoleRepo.ImaDozvolu(r.Context(), k.Uloga, "tema.globalno") {
		http.Error(w, "Nemate dozvolu za ovu akciju.", http.StatusForbidden)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	tema := r.FormValue("globalna_tema")
	validne := map[string]bool{"tamna": true, "svetla": true}
	if !validne[tema] {
		tema = "tamna"
	}

	if err := ntechsqlite.SacuvajPodesavanje(r.Context(), h.DB, "globalna_tema", tema); err != nil {
		http.Error(w, "Greška pri čuvanju teme", http.StatusInternalServerError)
		return
	}

	// poništi lokalne teme svih korisnika — tema se primenjuje globalno
	if _, err := h.DB.ExecContext(r.Context(),
		"UPDATE korisnici SET koristi_lokalnu_temu = 0, lokalna_tema = ''",
	); err != nil {
		log.Printf("PrimeniGlobalnuTemu: reset lokalnih tema: %v", err)
	}

	middleware.SetFlash(w, r, h.DB, "uspeh", "Globalna tema primenjena na sve korisnike.")
	http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
}
