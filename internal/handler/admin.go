package handler

import (
	"html/template"
	"net/http"
	"strconv"

	"ntech/internal/auth"
	"ntech/internal/db/sqlite"
	"ntech/internal/middleware"
	"ntech/internal/model"

	"github.com/go-chi/chi/v5"
)

type podaciAdminKorisnici struct {
	model.PodaciStranice
	Korisnici []model.Korisnik
	Greska    string
	Sacuvano  bool
}

type podaciAdminProfil struct {
	model.PodaciStranice
	Greska      string
	Sacuvano    string
	TotpURI     string
	TotpTajna   string
	TotpQR      template.URL
	TotpAktivan bool
}

// AdminKorisnici prikazuje listu korisnika
func (h *Handler) AdminKorisnici(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if !middleware.JeAdmin(k) {
		http.Error(w, "Pristup odbijen", http.StatusForbidden)
		return
	}

	podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	lista, err := h.KorisniciRepo.Lista(r.Context())
	if err != nil {
		http.Error(w, "Greška pri učitavanju korisnika", http.StatusInternalServerError)
		return
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "admin"
	ps.NaslovStranice = "Korisnici"

	podaci := podaciAdminKorisnici{
		PodaciStranice: ps,
		Korisnici:      lista,
		Greska:         r.URL.Query().Get("greska"),
		Sacuvano:       r.URL.Query().Get("sacuvano") == "1",
	}

	h.renderujTemplate(w, "admin_korisnici", podaci)
}

// AdminSacuvajKorisnika kreira novog korisnika
func (h *Handler) AdminSacuvajKorisnika(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if !middleware.JeAdmin(k) {
		http.Error(w, "Pristup odbijen", http.StatusForbidden)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin/korisnici?greska=1", http.StatusSeeOther)
		return
	}

	ime := r.FormValue("korisnicko_ime")
	lozinka := r.FormValue("lozinka")
	uloga := r.FormValue("uloga")

	validneUloge := map[string]bool{"superadmin": true, "admin": true, "radnik": true}
	if len(ime) < 3 || len(lozinka) < 8 || !validneUloge[uloga] {
		http.Redirect(w, r, "/admin/korisnici?greska=1", http.StatusSeeOther)
		return
	}

	// superadmin može kreirati samo admin i radnik (ne drugog superadmina) osim ako je jedini superadmin
	if uloga == "superadmin" && k.Uloga != "superadmin" {
		http.Error(w, "Pristup odbijen", http.StatusForbidden)
		return
	}

	hash, err := auth.HashujLozinku(lozinka)
	if err != nil {
		http.Redirect(w, r, "/admin/korisnici?greska=2", http.StatusSeeOther)
		return
	}

	if _, err := h.KorisniciRepo.Kreiraj(r.Context(), ime, hash, uloga); err != nil {
		http.Redirect(w, r, "/admin/korisnici?greska=2", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin/korisnici?sacuvano=1", http.StatusSeeOther)
}

// AdminToggleAktivan menja aktivan status korisnika
func (h *Handler) AdminToggleAktivan(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if !middleware.JeAdmin(k) {
		http.Error(w, "Pristup odbijen", http.StatusForbidden)
		return
	}

	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Redirect(w, r, "/admin/korisnici?greska=1", http.StatusSeeOther)
		return
	}

	// ne sme deaktivirati sam sebe
	if id == k.ID {
		http.Redirect(w, r, "/admin/korisnici?greska=1", http.StatusSeeOther)
		return
	}

	korisnik, err := h.KorisniciRepo.DohvatiPoID(r.Context(), id)
	if err != nil {
		http.Redirect(w, r, "/admin/korisnici?greska=1", http.StatusSeeOther)
		return
	}

	if err := h.KorisniciRepo.AzurirajAktivan(r.Context(), id, !korisnik.Aktivan); err != nil {
		http.Redirect(w, r, "/admin/korisnici?greska=2", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin/korisnici?sacuvano=1", http.StatusSeeOther)
}

// AdminPromeniUlogu menja ulogu korisnika
func (h *Handler) AdminPromeniUlogu(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if k == nil || k.Uloga != "superadmin" {
		http.Error(w, "Pristup odbijen", http.StatusForbidden)
		return
	}

	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Redirect(w, r, "/admin/korisnici?greska=1", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin/korisnici?greska=1", http.StatusSeeOther)
		return
	}

	uloga := r.FormValue("uloga")
	validneUloge := map[string]bool{"superadmin": true, "admin": true, "radnik": true}
	if !validneUloge[uloga] {
		http.Redirect(w, r, "/admin/korisnici?greska=1", http.StatusSeeOther)
		return
	}

	if err := h.KorisniciRepo.AzurirajUlogu(r.Context(), id, uloga); err != nil {
		http.Redirect(w, r, "/admin/korisnici?greska=2", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin/korisnici?sacuvano=1", http.StatusSeeOther)
}

// AdminProfil prikazuje stranicu profila
func (h *Handler) AdminProfil(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if k == nil {
		http.Redirect(w, r, "/prijava", http.StatusSeeOther)
		return
	}
	// osvežavamo korisnika iz baze da bismo imali aktuelni totp_tajna
	svezi, err := h.KorisniciRepo.DohvatiPoID(r.Context(), k.ID)
	if err != nil {
		http.Error(w, "Greška pri učitavanju profila", http.StatusInternalServerError)
		return
	}

	podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "admin"
	ps.NaslovStranice = "Moj profil"

	podaci := podaciAdminProfil{
		PodaciStranice: ps,
		Greska:         r.URL.Query().Get("greska"),
		Sacuvano:       r.URL.Query().Get("sacuvano"),
		TotpAktivan:    svezi.TotpTajna != "",
	}

	h.renderujTemplate(w, "admin_profil", podaci)
}

// AdminPromeniLozinku menja lozinku prijavljenog korisnika
func (h *Handler) AdminPromeniLozinku(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if k == nil {
		http.Redirect(w, r, "/prijava", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin/profil?greska=1", http.StatusSeeOther)
		return
	}

	stara := r.FormValue("stara_lozinka")
	nova := r.FormValue("nova_lozinka")
	potvrda := r.FormValue("nova_lozinka_potvrda")

	if !auth.ProveriLozinku(k.LozinkaHash, stara) {
		http.Redirect(w, r, "/admin/profil?greska=lozinka", http.StatusSeeOther)
		return
	}
	if len(nova) < 8 || nova != potvrda {
		http.Redirect(w, r, "/admin/profil?greska=lozinka2", http.StatusSeeOther)
		return
	}

	hash, err := auth.HashujLozinku(nova)
	if err != nil {
		http.Redirect(w, r, "/admin/profil?greska=2", http.StatusSeeOther)
		return
	}

	if err := h.KorisniciRepo.PromeniLozinku(r.Context(), k.ID, hash); err != nil {
		http.Redirect(w, r, "/admin/profil?greska=2", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin/profil?sacuvano=lozinka", http.StatusSeeOther)
}

// AdminTotpPokreni generiše TOTP tajnu i prikazuje QR kod
func (h *Handler) AdminTotpPokreni(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if k == nil {
		http.Redirect(w, r, "/prijava", http.StatusSeeOther)
		return
	}

	podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	totp, err := auth.GenerisuTotpTajnu(k.KorisnickoIme, podesavanja["naziv_firme"])
	if err != nil {
		http.Redirect(w, r, "/admin/profil?greska=2", http.StatusSeeOther)
		return
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "admin"
	ps.NaslovStranice = "Podesi 2FA"

	podaci := podaciAdminProfil{
		PodaciStranice: ps,
		TotpURI:        totp.URI,
		TotpTajna:      totp.Tajna,
		TotpQR:         template.URL("data:image/png;base64," + totp.QRBase64),
	}

	h.renderujTemplate(w, "admin_profil", podaci)
}

// AdminTotpAktivacija verifikuje TOTP kod i čuva tajnu
func (h *Handler) AdminTotpAktivacija(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if k == nil {
		http.Redirect(w, r, "/prijava", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin/profil?greska=1", http.StatusSeeOther)
		return
	}

	tajna := r.FormValue("totp_tajna")
	kod := r.FormValue("kod")

	if !auth.VerifikujTotpKod(kod, tajna) {
		// ponovni prikaz sa greškom — regenerišemo isti tajnu
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		ps := h.popuniPodaciStranice(r, podesavanja)
		ps.Stranica = "admin"
		ps.NaslovStranice = "Podesi 2FA"

		// regenerišemo QR za već generisanu tajnu (korisnik je video ovaj QR)
		nazivFirme := podesavanja["naziv_firme"]
		if nazivFirme == "" {
			nazivFirme = "NTech"
		}
		uri, qr := auth.RegenerisiTotpQR(tajna, k.KorisnickoIme, nazivFirme)

		podaci := podaciAdminProfil{
			PodaciStranice: ps,
			TotpURI:        uri,
			TotpTajna:      tajna,
			TotpQR:         template.URL("data:image/png;base64," + qr),
			Greska:         "totp",
		}
		h.renderujTemplate(w, "admin_profil", podaci)
		return
	}

	if err := h.KorisniciRepo.SacuvajTotpTajnu(r.Context(), k.ID, tajna); err != nil {
		http.Redirect(w, r, "/admin/profil?greska=2", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin/profil?sacuvano=totp", http.StatusSeeOther)
}

// AdminTotpDeaktivacija uklanja TOTP tajnu
func (h *Handler) AdminTotpDeaktivacija(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if k == nil {
		http.Redirect(w, r, "/prijava", http.StatusSeeOther)
		return
	}

	if err := h.KorisniciRepo.SacuvajTotpTajnu(r.Context(), k.ID, ""); err != nil {
		http.Redirect(w, r, "/admin/profil?greska=2", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin/profil?sacuvano=totp_off", http.StatusSeeOther)
}

// parseBoolForm čita boolean vrednost iz forme
func parseBoolForm(s string) bool {
	b, _ := strconv.ParseBool(s)
	return b
}
