package handler

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
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
)

// PodaciPodesavanja su podaci za stranicu podešavanja
type PodaciPodesavanja struct {
	model.PodaciStranice
	NazivFirme  string
	Podnazlov   string
	Adresa      string
	Telefon     string
	PIB         string
	LogoPutanja        string
	TopbarLogoSlika    bool
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
	podesavanja, err := ntechsqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
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

		novaDB, err := ntechsqlite.OtvoriDB(h.PutanjaBaze)
		if err != nil {
			slog.Error("vrati backup: greška pri otvaranju nove baze (potreban restart)", "error", err)
			return
		}
		h.reinicijalizujRepozitorijume(novaDB)
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

// SacuvajPodesavanja prima POST i čuva podešavanja u bazu
func (h *Handler) SacuvajPodesavanja(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "podesavanja.izmeni"); !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}

	// checkbox-i: šalju vrednost samo kada su čekirani, pa ih uvek eksplicitno čitamo
	topbarLogoSlika := "0"
	if r.FormValue("topbar_logo_slika") == "1" {
		topbarLogoSlika = "1"
	}

	polja := map[string]string{
		"naziv_firme":       r.FormValue("naziv_firme"),
		"podnazlov":         r.FormValue("podnazlov"),
		"adresa":            r.FormValue("adresa"),
		"telefon":           r.FormValue("telefon"),
		"pib":               r.FormValue("pib"),
		"topbar_logo_slika": topbarLogoSlika,
		// profil firme (Faza 0) — radio dugmad uvek šalju vrednost, pa se uredno čuvaju
		"firma_pravni_oblik":  r.FormValue("firma_pravni_oblik"),
		"firma_pdv_obveznik":  r.FormValue("firma_pdv_obveznik"),
		"firma_fiskalizacija": r.FormValue("firma_fiskalizacija"),
		"firma_rezim":         r.FormValue("firma_rezim"),
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

	// whitelist dozvoljenih redirekcija — vrednost uvek dolazi iz mape, nikad od korisnika
	dozvoljeniSledeci := map[string]string{
		"/admin/podesavanja/opste": "/admin/podesavanja/opste",
		"/podesavanja":             "/podesavanja",
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
		if err := ntechsqlite.SacuvajPodesavanje(r.Context(), h.DB, "backup_interval_sati", strconv.Itoa(n)); err != nil {
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
		if err := ntechsqlite.SacuvajPodesavanje(r.Context(), h.DB, "backup_broj_kopija", strconv.Itoa(n)); err != nil {
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
		if err := ntechsqlite.SacuvajPodesavanje(r.Context(), h.DB, "kalkulacija_marza", strconv.FormatFloat(marza, 'f', -1, 64)); err != nil {
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
		slog.Error("upload loga: ne mogu kreirati fajl", "error", err)
		http.Redirect(w, r, "/podesavanja?logo_greska=Greška+pri+čuvanju+fajla", http.StatusSeeOther)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, fajl); err != nil {
		slog.Error("upload loga: greška pri kopiranju", "error", err)
		http.Redirect(w, r, "/podesavanja?logo_greska=Greška+pri+čuvanju+fajla", http.StatusSeeOther)
		return
	}

	// timestamp u URL-u sprečava browser da koristi staru keširanu sliku
	putanja := fmt.Sprintf("/static/uploads/logo%s?v=%d", ext, time.Now().Unix())
	if err := ntechsqlite.SacuvajPodesavanje(r.Context(), h.DB, "logo_putanja", putanja); err != nil {
		slog.Error("upload loga: greška pri čuvanju putanje", "error", err)
		http.Redirect(w, r, "/podesavanja?logo_greska=Greška+pri+čuvanju+podešavanja", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/podesavanja?sacuvano=1", http.StatusSeeOther)
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
	if err := ntechsqlite.SacuvajPodesavanje(r.Context(), h.DB, "logo_putanja", ""); err != nil {
		slog.Error("ukloni logo: greška pri čuvanju", "error", err)
	}
	http.Redirect(w, r, "/admin/podesavanja/opste?sacuvano=1", http.StatusSeeOther)
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
	staraPodesavanja, _ := ntechsqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if stara := staraPodesavanja["login_pozadina"]; stara != "" {
		// putanja u bazi je oblika /static/uploads/ime.ext?v=..., izvlačimo samo ime fajla
		deoBezverzije := strings.Split(stara, "?")[0]
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
	if err := ntechsqlite.SacuvajPodesavanje(r.Context(), h.DB, "login_pozadina", putanja); err != nil {
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

	podesavanja, err := ntechsqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err == nil {
		if stara := podesavanja["login_pozadina"]; stara != "" {
			deoBezverzije := strings.Split(stara, "?")[0]
			staroIme := filepath.Base(deoBezverzije)
			os.Remove(filepath.Join("web/static/uploads", staroIme))
		}
	}

	if err := ntechsqlite.SacuvajPodesavanje(r.Context(), h.DB, "login_pozadina", ""); err != nil {
		slog.Error("ukloni login pozadinu: greška pri čuvanju", "error", err)
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri uklanjanju slike.")
		http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
		return
	}

	middleware.SetFlash(w, r, h.DB, "uspeh", "Pozadinska slika je uklonjena.")
	http.Redirect(w, r, "/admin/podesavanja/izgled", http.StatusSeeOther)
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
		if err := ntechsqlite.SacuvajPodesavanje(r.Context(), h.DB, kljuc, vrednost); err != nil {
			slog.Error("greška pri čuvanju stila login pozadine", "kljuc", kljuc, "error", err)
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
		PodaciStranice:                  ps,
		NazivFirme:                      podesavanja["naziv_firme"],
		Podnazlov:                       podesavanja["podnazlov"],
		Adresa:                          podesavanja["adresa"],
		Telefon:                         podesavanja["telefon"],
		PIB:                             podesavanja["pib"],
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
