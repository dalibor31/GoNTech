package handler

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ntech/internal/db/sqlite"
	"ntech/internal/middleware"
)

// sacuvanoReferer vraća URL za redirect posle uspešnog čuvanja:
// koristi Referer zaglavlje (samo putanja, nikad spoljni sajt) i dodaje ?sacuvano=1.
// AJAX handler u browseru detektuje sacuvano=1 i prikazuje toast bez reloada.
func sacuvanoReferer(r *http.Request, rezervna string) string {
	if ref := r.Referer(); ref != "" {
		if u, err := url.Parse(ref); err == nil {
			putanja := u.RequestURI()
			if strings.Contains(putanja, "?") {
				return putanja + "&sacuvano=1"
			}
			return putanja + "?sacuvano=1"
		}
	}
	return rezervna + "?sacuvano=1"
}

// ProfilTema prikazuje stranicu lične teme i pozadine
func (h *Handler) ProfilTema(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if !h.DozvoleRepo.ImaDozvolu(r.Context(), k.Uloga, "tema.lokalno") {
		http.Error(w, "Pristup odbijen.", http.StatusForbidden)
		return
	}

	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	svezi, err := h.KorisniciRepo.DohvatiPoID(r.Context(), k.ID)
	if err != nil {
		http.Error(w, "Greška pri učitavanju profila", http.StatusInternalServerError)
		return
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "profil-tema"
	ps.NaslovStranice = "Moja tema"

	podaci := podaciProfilTema{
		PodaciStranice:              ps,
		LokalnaTema:                 svezi.LokalnaTema,
		KoristiLokalnuTemu:          svezi.KoristiLokalnuTemu,
		LokalnaPozadina:             svezi.LokalnaPozadina,
		LokalnaPozadinaOpacity:      svezi.LokalnaPozadinaOpacity,
		LokalnaPozadinaBlur:         svezi.LokalnaPozadinaBlur,
		LokalnaPozadinaBlurPozadine: svezi.LokalnaPozadinaBlurPozadine,
		LokalnaPozadinaGlassOpacity: svezi.LokalnaPozadinaGlassOpacity,
		LokalnaAnimacija:            svezi.LokalnaAnimacija,
		LokalniHover:                svezi.LokalniHover,
	}
	if podaci.LokalnaPozadinaOpacity == "" {
		podaci.LokalnaPozadinaOpacity = "50"
	}
	if podaci.LokalnaPozadinaBlur == "" {
		podaci.LokalnaPozadinaBlur = "12"
	}
	if podaci.LokalnaPozadinaBlurPozadine == "" {
		podaci.LokalnaPozadinaBlurPozadine = "0"
	}
	if podaci.LokalnaPozadinaGlassOpacity == "" {
		podaci.LokalnaPozadinaGlassOpacity = "10"
	}
	h.renderujTemplate(w, "profil_tema", podaci)
}

// ProfilOtpremiPozadinu prima upload lične pozadinske slike
func (h *Handler) ProfilOtpremiPozadinu(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if !h.DozvoleRepo.ImaDozvolu(r.Context(), k.Uloga, "tema.lokalno") {
		http.Error(w, "Pristup odbijen.", http.StatusForbidden)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 5<<20+4096)
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Fajl je prevelik (maksimum 5 MB).")
		http.Redirect(w, r, "/profil/tema", http.StatusSeeOther)
		return
	}

	fajl, zaglavlje, err := r.FormFile("lokalna_pozadina")
	if err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Nije odabran fajl.")
		http.Redirect(w, r, "/profil/tema", http.StatusSeeOther)
		return
	}
	defer fajl.Close()

	if zaglavlje.Size > 5<<20 {
		middleware.SetFlash(w, r, h.DB, "greska", "Fajl je prevelik (maksimum 5 MB).")
		http.Redirect(w, r, "/profil/tema", http.StatusSeeOther)
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
		http.Redirect(w, r, "/profil/tema", http.StatusSeeOther)
		return
	}

	buf := make([]byte, 512)
	n, _ := fajl.Read(buf)
	stvarniMime := http.DetectContentType(buf[:n])
	if !strings.HasPrefix(stvarniMime, ocekivaniMime) {
		middleware.SetFlash(w, r, h.DB, "greska", "Sadržaj fajla ne odgovara odabranoj ekstenziji.")
		http.Redirect(w, r, "/profil/tema", http.StatusSeeOther)
		return
	}
	if _, err := fajl.Seek(0, io.SeekStart); err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri obradi fajla.")
		http.Redirect(w, r, "/profil/tema", http.StatusSeeOther)
		return
	}

	// briše staru ličnu pozadinu sa diska ako postoji
	svezi, _ := h.KorisniciRepo.DohvatiPoID(r.Context(), k.ID)
	if svezi != nil && svezi.LokalnaPozadina != "" {
		deo, _, _ := strings.Cut(svezi.LokalnaPozadina, "?")
		os.Remove(filepath.Join("web/static/uploads", filepath.Base(deo)))
	}

	// ime fajla: korisnik_{id}_pozadina.ext — deterministično, lako obrisati
	novoIme := fmt.Sprintf("korisnik_%d_pozadina%s", k.ID, ext)
	odrediste := filepath.Join("web/static/uploads", novoIme)
	dst, err := os.Create(odrediste)
	if err != nil {
		slog.Error("ProfilOtpremiPozadinu: ne mogu kreirati fajl", "error", err)
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju fajla.")
		http.Redirect(w, r, "/profil/tema", http.StatusSeeOther)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, fajl); err != nil {
		slog.Error("ProfilOtpremiPozadinu: greška pri kopiranju", "error", err)
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju fajla.")
		http.Redirect(w, r, "/profil/tema", http.StatusSeeOther)
		return
	}

	putanja := fmt.Sprintf("/static/uploads/%s?v=%d", novoIme, time.Now().Unix())

	opacity := "50"
	blur := "12"
	blurPozadine := "0"
	glassOpacity := "10"
	if svezi != nil {
		if svezi.LokalnaPozadinaOpacity != "" {
			opacity = svezi.LokalnaPozadinaOpacity
		}
		if svezi.LokalnaPozadinaBlur != "" {
			blur = svezi.LokalnaPozadinaBlur
		}
		if svezi.LokalnaPozadinaBlurPozadine != "" {
			blurPozadine = svezi.LokalnaPozadinaBlurPozadine
		}
		if svezi.LokalnaPozadinaGlassOpacity != "" {
			glassOpacity = svezi.LokalnaPozadinaGlassOpacity
		}
	}

	if err := h.KorisniciRepo.SacuvajLokalnuPozadinu(r.Context(), k.ID, putanja, opacity, blur, blurPozadine, glassOpacity); err != nil {
		slog.Error("ProfilOtpremiPozadinu: greška pri čuvanju", "error", err)
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju podešavanja.")
		http.Redirect(w, r, "/profil/tema", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/profil/tema?sacuvano=1", http.StatusSeeOther)
}

// ProfilUkloniPozadinu briše ličnu pozadinsku sliku korisnika
func (h *Handler) ProfilUkloniPozadinu(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if !h.DozvoleRepo.ImaDozvolu(r.Context(), k.Uloga, "tema.lokalno") {
		http.Error(w, "Pristup odbijen.", http.StatusForbidden)
		return
	}

	svezi, _ := h.KorisniciRepo.DohvatiPoID(r.Context(), k.ID)
	if svezi != nil && svezi.LokalnaPozadina != "" {
		deo, _, _ := strings.Cut(svezi.LokalnaPozadina, "?")
		os.Remove(filepath.Join("web/static/uploads", filepath.Base(deo)))
	}

	if err := h.KorisniciRepo.SacuvajLokalnuPozadinu(r.Context(), k.ID, "", "50", "12", "0", "10"); err != nil {
		slog.Error("ProfilUkloniPozadinu", "error", err)
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri uklanjanju slike.")
		http.Redirect(w, r, "/profil/tema", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/profil/tema?sacuvano=1", http.StatusSeeOther)
}

// ProfilSacuvajPozadinuStilove čuva opacity i blur lične pozadine
func (h *Handler) ProfilSacuvajPozadinuStilove(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if !h.DozvoleRepo.ImaDozvolu(r.Context(), k.Uloga, "tema.lokalno") {
		http.Error(w, "Pristup odbijen.", http.StatusForbidden)
		return
	}

	if err := r.ParseForm(); err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čitanju forme.")
		http.Redirect(w, r, "/profil/tema", http.StatusSeeOther)
		return
	}

	svezi, _ := h.KorisniciRepo.DohvatiPoID(r.Context(), k.ID)
	pozadina := ""
	if svezi != nil {
		pozadina = svezi.LokalnaPozadina
	}

	opacity := r.FormValue("lokalna_pozadina_opacity")
	blur := r.FormValue("lokalna_pozadina_blur")
	blurPozadineSt := r.FormValue("lokalna_pozadina_blur_pozadine")
	glassOpacitySt := r.FormValue("lokalna_pozadina_glass_opacity")
	if opacity == "" {
		opacity = "50"
	}
	if blur == "" {
		blur = "12"
	}
	if blurPozadineSt == "" {
		blurPozadineSt = "0"
	}
	if glassOpacitySt == "" {
		glassOpacitySt = "10"
	}

	if err := h.KorisniciRepo.SacuvajLokalnuPozadinu(r.Context(), k.ID, pozadina, opacity, blur, blurPozadineSt, glassOpacitySt); err != nil {
		slog.Error("ProfilSacuvajPozadinuStilove", "error", err)
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju podešavanja.")
		http.Redirect(w, r, "/profil/tema", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/profil/tema?sacuvano=1", http.StatusSeeOther)
}

// SacuvajLokalnuTemu čuva korisnikovu lokalnu temu
func (h *Handler) SacuvajLokalnuTemu(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if k == nil {
		http.Redirect(w, r, "/prijava", http.StatusSeeOther)
		return
	}
	if !h.DozvoleRepo.ImaDozvolu(r.Context(), k.Uloga, "tema.lokalno") {
		http.Error(w, "Pristup odbijen", http.StatusForbidden)
		return
	}

	if err := r.ParseForm(); err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Greška. Pokušajte ponovo.")
		http.Redirect(w, r, "/admin/profil", http.StatusSeeOther)
		return
	}

	koristi := r.FormValue("koristi_lokalnu_temu") == "1"
	lokalnaTema := r.FormValue("lokalna_tema")
	if lokalnaTema != "tamna" && lokalnaTema != "svetla" {
		lokalnaTema = "tamna"
	}

	if err := h.KorisniciRepo.SacuvajLokalnuTemu(r.Context(), k.ID, lokalnaTema, koristi); err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju. Pokušajte ponovo.")
		http.Redirect(w, r, "/admin/profil", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, sacuvanoReferer(r, "/admin/profil"), http.StatusSeeOther)
}

// SacuvajLokalnuAnimaciju čuva korisnikovu preferencu animacije tabela
func (h *Handler) SacuvajLokalnuAnimaciju(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if k == nil {
		http.Redirect(w, r, "/prijava", http.StatusSeeOther)
		return
	}
	if !h.DozvoleRepo.ImaDozvolu(r.Context(), k.Uloga, "tema.lokalno") {
		http.Error(w, "Pristup odbijen", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Greška. Pokušajte ponovo.")
		http.Redirect(w, r, "/admin/profil/tema", http.StatusSeeOther)
		return
	}

	animacija := r.FormValue("lokalna_animacija")
	dozvoljene := map[string]bool{"": true, "bez": true, "fadeInUp": true, "fadeIn": true, "blurIn": true, "slideLeft": true}
	if !dozvoljene[animacija] {
		animacija = ""
	}

	brzina := r.FormValue("lokalna_brzina_animacije")
	dozvoljenieBrzine := map[string]bool{"": true, "0.1": true, "0.2": true, "0.3": true, "0.4": true, "0.5": true, "0.6": true, "0.7": true, "0.8": true}
	if !dozvoljenieBrzine[brzina] {
		brzina = ""
	}

	if err := h.KorisniciRepo.SacuvajLokalnuAnimaciju(r.Context(), k.ID, animacija); err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju. Pokušajte ponovo.")
		http.Redirect(w, r, "/admin/profil/tema", http.StatusSeeOther)
		return
	}
	if err := h.KorisniciRepo.SacuvajLokalnuBrzinuAnimacije(r.Context(), k.ID, brzina); err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju. Pokušajte ponovo.")
		http.Redirect(w, r, "/admin/profil/tema", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, sacuvanoReferer(r, "/admin/profil/tema"), http.StatusSeeOther)
}

// SacuvajLokalniHover čuva korisnikovu preferencu hover efekta na karticama
func (h *Handler) SacuvajLokalniHover(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if k == nil {
		http.Redirect(w, r, "/prijava", http.StatusSeeOther)
		return
	}
	if !h.DozvoleRepo.ImaDozvolu(r.Context(), k.Uloga, "tema.lokalno") {
		http.Error(w, "Pristup odbijen", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Greška. Pokušajte ponovo.")
		http.Redirect(w, r, "/admin/profil/tema", http.StatusSeeOther)
		return
	}

	hover := r.FormValue("lokalni_hover")
	dozvoljeni := map[string]bool{"": true, "bez": true, "podizanje": true, "svetlost": true, "zoom": true, "boja": true}
	if !dozvoljeni[hover] {
		hover = ""
	}

	if err := h.KorisniciRepo.SacuvajLokalniHover(r.Context(), k.ID, hover); err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju. Pokušajte ponovo.")
		http.Redirect(w, r, "/admin/profil/tema", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, sacuvanoReferer(r, "/admin/profil/tema"), http.StatusSeeOther)
}

// SacuvajLokalnuBrzinuAnimacije čuva korisnikovu preferencu brzine animacije tabela
func (h *Handler) SacuvajLokalnuBrzinuAnimacije(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if k == nil {
		http.Redirect(w, r, "/prijava", http.StatusSeeOther)
		return
	}
	if !h.DozvoleRepo.ImaDozvolu(r.Context(), k.Uloga, "tema.lokalno") {
		http.Error(w, "Pristup odbijen", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Greška. Pokušajte ponovo.")
		http.Redirect(w, r, "/admin/profil/tema", http.StatusSeeOther)
		return
	}

	brzina := r.FormValue("lokalna_brzina_animacije")
	dozvoljene := map[string]bool{"": true, "0.2": true, "0.4": true, "0.6": true, "0.8": true, "1.0": true, "1.2": true, "1.5": true}
	if !dozvoljene[brzina] {
		brzina = ""
	}

	if err := h.KorisniciRepo.SacuvajLokalnuBrzinuAnimacije(r.Context(), k.ID, brzina); err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju. Pokušajte ponovo.")
		http.Redirect(w, r, "/admin/profil/tema", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, sacuvanoReferer(r, "/admin/profil/tema"), http.StatusSeeOther)
}

// ProfilOtpremiAvatar prima upload lične avatar slike korisnika
func (h *Handler) ProfilOtpremiAvatar(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if k == nil {
		http.Redirect(w, r, "/prijava", http.StatusSeeOther)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 2<<20+4096)
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Fajl je prevelik (maksimum 2 MB).")
		http.Redirect(w, r, "/profil/tema", http.StatusSeeOther)
		return
	}

	fajl, zaglavlje, err := r.FormFile("avatar")
	if err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Nije odabran fajl.")
		http.Redirect(w, r, "/profil/tema", http.StatusSeeOther)
		return
	}
	defer fajl.Close()

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
		http.Redirect(w, r, "/profil/tema", http.StatusSeeOther)
		return
	}

	buf := make([]byte, 512)
	n, err := fajl.Read(buf)
	if err != nil && err != io.EOF {
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čitanju fajla.")
		http.Redirect(w, r, "/profil/tema", http.StatusSeeOther)
		return
	}
	stvarniMime := http.DetectContentType(buf[:n])
	if !strings.HasPrefix(stvarniMime, ocekivaniMime) {
		middleware.SetFlash(w, r, h.DB, "greska", "Sadržaj fajla ne odgovara odabranoj ekstenziji.")
		http.Redirect(w, r, "/profil/tema", http.StatusSeeOther)
		return
	}
	if _, err := fajl.Seek(0, io.SeekStart); err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri obradi fajla.")
		http.Redirect(w, r, "/profil/tema", http.StatusSeeOther)
		return
	}

	// briše stari avatar sa diska
	svezi, _ := h.KorisniciRepo.DohvatiPoID(r.Context(), k.ID)
	if svezi != nil && svezi.AvatarPutanja != "" {
		deo, _, _ := strings.Cut(svezi.AvatarPutanja, "?")
		os.Remove(filepath.Join("web/static/uploads", filepath.Base(deo)))
	}

	novoIme := fmt.Sprintf("korisnik_%d_avatar%s", k.ID, ext)
	odrediste := filepath.Join("web/static/uploads", novoIme)
	dst, err := os.Create(odrediste)
	if err != nil {
		slog.Error("ProfilOtpremiAvatar: ne mogu kreirati fajl", "error", err)
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju fajla.")
		http.Redirect(w, r, "/profil/tema", http.StatusSeeOther)
		return
	}
	defer dst.Close()
	if _, err := io.Copy(dst, fajl); err != nil {
		slog.Error("ProfilOtpremiAvatar: greška pri kopiranju", "error", err)
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju fajla.")
		http.Redirect(w, r, "/profil/tema", http.StatusSeeOther)
		return
	}

	// cache-buster verzija u URL-u
	v := fmt.Sprintf("%d", time.Now().Unix())
	putanja := fmt.Sprintf("/static/uploads/%s?v=%s", novoIme, v)

	if err := h.KorisniciRepo.SacuvajAvatar(r.Context(), k.ID, putanja); err != nil {
		slog.Error("ProfilOtpremiAvatar: greška pri čuvanju u bazi", "error", err)
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju.")
		http.Redirect(w, r, "/profil/tema", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/profil/tema?sacuvano=1", http.StatusSeeOther)
}

// ProfilUkloniAvatar briše ličnu avatar sliku korisnika
func (h *Handler) ProfilUkloniAvatar(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if k == nil {
		http.Redirect(w, r, "/prijava", http.StatusSeeOther)
		return
	}

	svezi, _ := h.KorisniciRepo.DohvatiPoID(r.Context(), k.ID)
	if svezi != nil && svezi.AvatarPutanja != "" {
		deo, _, _ := strings.Cut(svezi.AvatarPutanja, "?")
		os.Remove(filepath.Join("web/static/uploads", filepath.Base(deo)))
	}

	if err := h.KorisniciRepo.SacuvajAvatar(r.Context(), k.ID, ""); err != nil {
		slog.Error("ProfilUkloniAvatar", "error", err)
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri uklanjanju avatara.")
		http.Redirect(w, r, "/profil/tema", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/profil/tema?sacuvano=1", http.StatusSeeOther)
}
