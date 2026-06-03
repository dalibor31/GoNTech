package handler

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ntech/internal/db/sqlite"
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
	LogoGreska  string
}

// Podesavanja renderuje stranicu podešavanja
func (h *Handler) Podesavanja(w http.ResponseWriter, r *http.Request) {
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
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
		Verzija:        h.Verzija,
		LogoGreska:     r.URL.Query().Get("logo_greska"),
	}

	h.renderujTemplate(w, "podesavanja", podaci)
}

// SacuvajPodesavanja prima POST i čuva podešavanja u bazu
func (h *Handler) SacuvajPodesavanja(w http.ResponseWriter, r *http.Request) {
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
		if err := sqlite.SacuvajPodesavanje(r.Context(), h.DB, kljuc, vrednost); err != nil {
			http.Error(w, "Greška pri čuvanju podešavanja", http.StatusInternalServerError)
			return
		}
	}

	http.Redirect(w, r, "/podesavanja?sacuvano=1", http.StatusSeeOther)
}

// BackupBaze kreira konzistentnu kopiju baze i šalje je kao attachment
func (h *Handler) BackupBaze(w http.ResponseWriter, r *http.Request) {
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

	putanja := "/static/uploads/logo" + ext
	if err := sqlite.SacuvajPodesavanje(r.Context(), h.DB, "logo_putanja", putanja); err != nil {
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

	if err := sqlite.SacuvajPodesavanje(r.Context(), h.DB, "tema", tema); err != nil {
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
