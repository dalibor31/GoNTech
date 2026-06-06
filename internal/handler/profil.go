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
	"ntech/internal/middleware"
)

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
		PodaciStranice:      ps,
		LokalnaTema:         svezi.LokalnaTema,
		KoristiLokalnuTemu:  svezi.KoristiLokalnuTemu,
		KoristiGlobalnuTemu: !svezi.KoristiLokalnuTemu,
		GlobalnaTema:        podesavanja["globalna_tema"],
		LokalnaPozadina:             svezi.LokalnaPozadina,
		LokalnaPozadinaOpacity:      svezi.LokalnaPozadinaOpacity,
		LokalnaPozadinaBlur:         svezi.LokalnaPozadinaBlur,
		LokalnaPozadinaBlurPozadine: svezi.LokalnaPozadinaBlurPozadine,
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
		log.Printf("ProfilOtpremiPozadinu: ne mogu kreirati fajl: %v", err)
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju fajla.")
		http.Redirect(w, r, "/profil/tema", http.StatusSeeOther)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, fajl); err != nil {
		log.Printf("ProfilOtpremiPozadinu: greška pri kopiranju: %v", err)
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju fajla.")
		http.Redirect(w, r, "/profil/tema", http.StatusSeeOther)
		return
	}

	putanja := fmt.Sprintf("/static/uploads/%s?v=%d", novoIme, time.Now().Unix())

	opacity := "50"
	blur := "12"
	blurPozadine := "0"
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
	}

	if err := h.KorisniciRepo.SacuvajLokalnuPozadinu(r.Context(), k.ID, putanja, opacity, blur, blurPozadine); err != nil {
		log.Printf("ProfilOtpremiPozadinu: greška pri čuvanju: %v", err)
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju podešavanja.")
		http.Redirect(w, r, "/profil/tema", http.StatusSeeOther)
		return
	}

	middleware.SetFlash(w, r, h.DB, "uspeh", "Pozadinska slika je uspešno otpremljena.")
	http.Redirect(w, r, "/profil/tema", http.StatusSeeOther)
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

	if err := h.KorisniciRepo.SacuvajLokalnuPozadinu(r.Context(), k.ID, "", "50", "12", "0"); err != nil {
		log.Printf("ProfilUkloniPozadinu: %v", err)
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri uklanjanju slike.")
		http.Redirect(w, r, "/profil/tema", http.StatusSeeOther)
		return
	}

	middleware.SetFlash(w, r, h.DB, "uspeh", "Pozadinska slika je uklonjena.")
	http.Redirect(w, r, "/profil/tema", http.StatusSeeOther)
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
	if opacity == "" {
		opacity = "50"
	}
	if blur == "" {
		blur = "12"
	}
	if blurPozadineSt == "" {
		blurPozadineSt = "0"
	}

	if err := h.KorisniciRepo.SacuvajLokalnuPozadinu(r.Context(), k.ID, pozadina, opacity, blur, blurPozadineSt); err != nil {
		log.Printf("ProfilSacuvajPozadinuStilove: %v", err)
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju podešavanja.")
		http.Redirect(w, r, "/profil/tema", http.StatusSeeOther)
		return
	}

	middleware.SetFlash(w, r, h.DB, "uspeh", "Podešavanja su sačuvana.")
	http.Redirect(w, r, "/profil/tema", http.StatusSeeOther)
}
