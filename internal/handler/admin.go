package handler

import (
	"html/template"
	"net/http"

	"ntech/internal/auth"
	"ntech/internal/db/sqlite"
	"ntech/internal/middleware"
	"ntech/internal/model"

	"github.com/go-chi/chi/v5"
)

type podaciAdminKorisnici struct {
	model.PodaciStranice
	Korisnici []model.Korisnik
}

type podaciLoginIstorija struct {
	model.PodaciStranice
	PrikazKorisnik model.Korisnik
	Istorija       []*model.LoginPokusaj
}

type podaciAdminProfil struct {
	model.PodaciStranice
	// Greska se koristi samo za inline prikaz greške pri TOTP aktivaciji (bez redirecta)
	Greska             string
	TotpURI            string
	TotpTajna          string
	TotpQR             template.URL
	TotpAktivan        bool
	RezervniKodovi     []string // jednokratni prikaz novih kodova (posle aktivacije/regeneracije)
	BrojRezervnih      int      // koliko neiskorišćenih rezervnih kodova je preostalo
	LokalnaTema        string
	KoristiLokalnuTemu bool
	JelDemo            bool
}

type podaciProfilTema struct {
	model.PodaciStranice
	LokalnaTema                 string
	KoristiLokalnuTemu          bool
	LokalnaPozadina             string
	LokalnaPozadinaOpacity      string
	LokalnaPozadinaBlur         string
	LokalnaPozadinaBlurPozadine string
	LokalnaPozadinaGlassOpacity string
	LokalnaAnimacija            string
	LokalniHover                string
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

	// admin ne sme da vidi superadmin naloge — filtriramo ih iz liste
	if k.Uloga != "superadmin" {
		filtrirano := lista[:0]
		for _, kor := range lista {
			if kor.Uloga != "superadmin" {
				filtrirano = append(filtrirano, kor)
			}
		}
		lista = filtrirano
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "admin"
	ps.NaslovStranice = "Korisnici"

	h.renderujTemplate(w, "admin_korisnici", podaciAdminKorisnici{
		PodaciStranice: ps,
		Korisnici:      lista,
	})
}

// AdminSacuvajKorisnika kreira novog korisnika
func (h *Handler) AdminSacuvajKorisnika(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if !middleware.JeAdmin(k) {
		http.Error(w, "Pristup odbijen", http.StatusForbidden)
		return
	}

	if err := r.ParseForm(); err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Proverite unete podatke.")
		http.Redirect(w, r, "/admin/korisnici", http.StatusSeeOther)
		return
	}

	ime := r.FormValue("korisnicko_ime")
	lozinka := r.FormValue("lozinka")
	uloga := r.FormValue("uloga")

	// superadmin uloga se ne može kreirati kroz interfejs — jedini superadmin postoji od setup-a
	validneUloge := map[string]bool{"admin": true, "radnik": true}
	if len(ime) < 3 || len(lozinka) < 8 || !validneUloge[uloga] {
		middleware.SetFlash(w, r, h.DB, "greska", "Proverite unete podatke.")
		http.Redirect(w, r, "/admin/korisnici", http.StatusSeeOther)
		return
	}

	hash, err := auth.HashujLozinku(lozinka)
	if err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju. Pokušajte ponovo.")
		http.Redirect(w, r, "/admin/korisnici", http.StatusSeeOther)
		return
	}

	if _, err := h.KorisniciRepo.Kreiraj(r.Context(), ime, hash, uloga); err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju. Pokušajte ponovo.")
		http.Redirect(w, r, "/admin/korisnici", http.StatusSeeOther)
		return
	}

	middleware.SetFlash(w, r, h.DB, "uspeh", "Korisnik je uspešno kreiran.")
	http.Redirect(w, r, "/admin/korisnici", http.StatusSeeOther)
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
		middleware.SetFlash(w, r, h.DB, "greska", "Proverite unete podatke.")
		http.Redirect(w, r, "/admin/korisnici", http.StatusSeeOther)
		return
	}

	// ne sme deaktivirati sam sebe
	if id == k.ID {
		middleware.SetFlash(w, r, h.DB, "greska", "Ne možete promeniti status sopstvenog naloga.")
		http.Redirect(w, r, "/admin/korisnici", http.StatusSeeOther)
		return
	}

	korisnik, err := h.KorisniciRepo.DohvatiPoID(r.Context(), id)
	if err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Proverite unete podatke.")
		http.Redirect(w, r, "/admin/korisnici", http.StatusSeeOther)
		return
	}

	// admin ne sme da menja status drugog admina ni superadmina
	if korisnik.Uloga == "superadmin" || (korisnik.Uloga == "admin" && k.Uloga != "superadmin") {
		middleware.SetFlash(w, r, h.DB, "greska", "Ova radnja nije dozvoljena.")
		http.Redirect(w, r, "/admin/korisnici", http.StatusSeeOther)
		return
	}

	if err := h.KorisniciRepo.AzurirajAktivan(r.Context(), id, !korisnik.Aktivan); err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju. Pokušajte ponovo.")
		http.Redirect(w, r, "/admin/korisnici", http.StatusSeeOther)
		return
	}

	middleware.SetFlash(w, r, h.DB, "uspeh", "Promene su uspešno sačuvane.")
	http.Redirect(w, r, "/admin/korisnici", http.StatusSeeOther)
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
		middleware.SetFlash(w, r, h.DB, "greska", "Proverite unete podatke.")
		http.Redirect(w, r, "/admin/korisnici", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Proverite unete podatke.")
		http.Redirect(w, r, "/admin/korisnici", http.StatusSeeOther)
		return
	}

	// superadmin ne može menjati svoju vlastitu ulogu
	if id == k.ID {
		middleware.SetFlash(w, r, h.DB, "greska", "Ova radnja nije dozvoljena.")
		http.Redirect(w, r, "/admin/korisnici", http.StatusSeeOther)
		return
	}

	uloga := r.FormValue("uloga")
	// dozvoljena je samo promena između admin i radnik; superadmin uloga se ne može dodeliti ni ukloniti
	validneUloge := map[string]bool{"admin": true, "radnik": true}
	if !validneUloge[uloga] {
		middleware.SetFlash(w, r, h.DB, "greska", "Proverite unete podatke.")
		http.Redirect(w, r, "/admin/korisnici", http.StatusSeeOther)
		return
	}

	// dohvati korisnika da proverimo njegovu trenutnu ulogu
	ciljniKorisnik, err := h.KorisniciRepo.DohvatiPoID(r.Context(), id)
	if err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju. Pokušajte ponovo.")
		http.Redirect(w, r, "/admin/korisnici", http.StatusSeeOther)
		return
	}

	// superadmin uloga se ne može menjati
	if ciljniKorisnik.Uloga == "superadmin" {
		middleware.SetFlash(w, r, h.DB, "greska", "Ova radnja nije dozvoljena.")
		http.Redirect(w, r, "/admin/korisnici", http.StatusSeeOther)
		return
	}

	if err := h.KorisniciRepo.AzurirajUlogu(r.Context(), id, uloga); err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju. Pokušajte ponovo.")
		http.Redirect(w, r, "/admin/korisnici", http.StatusSeeOther)
		return
	}

	middleware.SetFlash(w, r, h.DB, "uspeh", "Promene su uspešno sačuvane.")
	http.Redirect(w, r, "/admin/korisnici", http.StatusSeeOther)
}

// AdminObrisiKorisnika briše korisnika sa ulogom radnik
func (h *Handler) AdminObrisiKorisnika(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if k == nil || k.Uloga != "superadmin" {
		http.Error(w, "Pristup odbijen", http.StatusForbidden)
		return
	}

	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Proverite unete podatke.")
		http.Redirect(w, r, "/admin/korisnici", http.StatusSeeOther)
		return
	}

	if id == k.ID {
		middleware.SetFlash(w, r, h.DB, "greska", "Ova radnja nije dozvoljena.")
		http.Redirect(w, r, "/admin/korisnici", http.StatusSeeOther)
		return
	}

	ciljni, err := h.KorisniciRepo.DohvatiPoID(r.Context(), id)
	if err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju. Pokušajte ponovo.")
		http.Redirect(w, r, "/admin/korisnici", http.StatusSeeOther)
		return
	}

	// dozvoljeno je brisanje samo radnika
	if ciljni.Uloga != "radnik" {
		middleware.SetFlash(w, r, h.DB, "greska", "Ova radnja nije dozvoljena.")
		http.Redirect(w, r, "/admin/korisnici", http.StatusSeeOther)
		return
	}

	if err := h.KorisniciRepo.Obrisi(r.Context(), id); err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju. Pokušajte ponovo.")
		http.Redirect(w, r, "/admin/korisnici", http.StatusSeeOther)
		return
	}

	middleware.SetFlash(w, r, h.DB, "uspeh", "Korisnik je obrisan.")
	http.Redirect(w, r, "/admin/korisnici", http.StatusSeeOther)
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
	ps.Stranica = "profil"
	ps.NaslovStranice = "Moj profil"

	var brojRezervnih int
	if svezi.TotpTajna != "" {
		brojRezervnih, _ = h.RezervniKodoviRepo.BrojPreostalih(r.Context(), svezi.ID)
	}

	h.renderujTemplate(w, "admin_profil", podaciAdminProfil{
		PodaciStranice:     ps,
		TotpAktivan:        svezi.TotpTajna != "",
		BrojRezervnih:      brojRezervnih,
		LokalnaTema:        svezi.LokalnaTema,
		KoristiLokalnuTemu: svezi.KoristiLokalnuTemu,
		JelDemo:            h.JelDemo,
	})
}

// generisiIPrikaziKodove generiše nove rezervne kodove, čuva njihove bcrypt hešove
// (poništavajući stare) i renderuje profil sa kodovima prikazanim JEDNOM. Čist
// tekst kodova se nigde ne čuva — korisnik mora da ih sačuva sada.
func (h *Handler) generisiIPrikaziKodove(w http.ResponseWriter, r *http.Request, k *model.Korisnik, poruka string) {
	kodovi, err := auth.GenerisiRezervneKodove(auth.BrojRezervnihKodova)
	if err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri generisanju rezervnih kodova.")
		http.Redirect(w, r, "/admin/profil", http.StatusSeeOther)
		return
	}
	hashevi := make([]string, 0, len(kodovi))
	for _, kod := range kodovi {
		hash, err := auth.HashujLozinku(kod)
		if err != nil {
			middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju rezervnih kodova.")
			http.Redirect(w, r, "/admin/profil", http.StatusSeeOther)
			return
		}
		hashevi = append(hashevi, hash)
	}
	if err := h.RezervniKodoviRepo.Zameni(r.Context(), k.ID, hashevi); err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju rezervnih kodova.")
		http.Redirect(w, r, "/admin/profil", http.StatusSeeOther)
		return
	}

	podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "profil"
	ps.NaslovStranice = "Moj profil"
	ps.Flash = &model.FlashPoruka{Tip: "uspeh", Poruka: poruka}

	h.renderujTemplate(w, "admin_profil", podaciAdminProfil{
		PodaciStranice: ps,
		TotpAktivan:    true,
		RezervniKodovi: kodovi,
		BrojRezervnih:  len(kodovi),
		JelDemo:        h.JelDemo,
	})
}

// AdminPromeniLozinku menja lozinku prijavljenog korisnika
func (h *Handler) AdminPromeniLozinku(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if k == nil {
		http.Redirect(w, r, "/prijava", http.StatusSeeOther)
		return
	}

	if h.JelDemo {
		middleware.SetFlash(w, r, h.DB, "greska", "Promena lozinke nije dozvoljena u demo modu.")
		http.Redirect(w, r, "/admin/profil", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Greška. Pokušajte ponovo.")
		http.Redirect(w, r, "/admin/profil", http.StatusSeeOther)
		return
	}

	stara := r.FormValue("stara_lozinka")
	nova := r.FormValue("nova_lozinka")
	potvrda := r.FormValue("nova_lozinka_potvrda")

	if !auth.ProveriLozinku(k.LozinkaHash, stara) {
		middleware.SetFlash(w, r, h.DB, "greska", "Stara lozinka nije ispravna.")
		http.Redirect(w, r, "/admin/profil", http.StatusSeeOther)
		return
	}
	if len(nova) < 8 || nova != potvrda {
		middleware.SetFlash(w, r, h.DB, "greska", "Nova lozinka mora imati najmanje 8 karaktera i lozinke moraju biti iste.")
		http.Redirect(w, r, "/admin/profil", http.StatusSeeOther)
		return
	}

	hash, err := auth.HashujLozinku(nova)
	if err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju. Pokušajte ponovo.")
		http.Redirect(w, r, "/admin/profil", http.StatusSeeOther)
		return
	}

	if err := h.KorisniciRepo.PromeniLozinku(r.Context(), k.ID, hash); err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju. Pokušajte ponovo.")
		http.Redirect(w, r, "/admin/profil", http.StatusSeeOther)
		return
	}

	middleware.SetFlash(w, r, h.DB, "uspeh", "Lozinka je uspešno promenjena.")
	http.Redirect(w, r, "/admin/profil", http.StatusSeeOther)
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
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri generisanju 2FA. Pokušajte ponovo.")
		http.Redirect(w, r, "/admin/profil", http.StatusSeeOther)
		return
	}

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "profil"
	ps.NaslovStranice = "Podesi 2FA"

	h.renderujTemplate(w, "admin_profil", podaciAdminProfil{
		PodaciStranice: ps,
		TotpURI:        totp.URI,
		TotpTajna:      totp.Tajna,
		TotpQR:         template.URL("data:image/png;base64," + totp.QRBase64),
		JelDemo:        h.JelDemo,
	})
}

// AdminTotpAktivacija verifikuje TOTP kod i čuva tajnu
func (h *Handler) AdminTotpAktivacija(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if k == nil {
		http.Redirect(w, r, "/prijava", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Greška. Pokušajte ponovo.")
		http.Redirect(w, r, "/admin/profil", http.StatusSeeOther)
		return
	}

	tajna := r.FormValue("totp_tajna")
	kod := r.FormValue("kod")

	if !auth.VerifikujTotpKod(kod, tajna) {
		// ponovni prikaz sa greškom — regenerišemo isti QR, ne radimo redirect
		podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
		ps := h.popuniPodaciStranice(r, podesavanja)
		ps.Stranica = "profil"
		ps.NaslovStranice = "Podesi 2FA"

		nazivFirme := podesavanja["naziv_firme"]
		if nazivFirme == "" {
			nazivFirme = "NTech"
		}
		uri, qr := auth.RegenerisiTotpQR(tajna, k.KorisnickoIme, nazivFirme)

		h.renderujTemplate(w, "admin_profil", podaciAdminProfil{
			PodaciStranice: ps,
			TotpURI:        uri,
			TotpTajna:      tajna,
			TotpQR:         template.URL("data:image/png;base64," + qr),
			Greska:         "totp",
			JelDemo:        h.JelDemo,
		})
		return
	}

	if err := h.KorisniciRepo.SacuvajTotpTajnu(r.Context(), k.ID, tajna); err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju. Pokušajte ponovo.")
		http.Redirect(w, r, "/admin/profil", http.StatusSeeOther)
		return
	}

	// uključenjem 2FA generišemo i rezervne kodove i prikazujemo ih jednom
	h.generisiIPrikaziKodove(w, r, k,
		"Dvostepena verifikacija je uključena. Sačuvajte rezervne kodove — prikazuju se samo sada.")
}

// AdminTotpRegenerisiKodove generiše nove rezervne kodove i poništava stare
func (h *Handler) AdminTotpRegenerisiKodove(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if k == nil {
		http.Redirect(w, r, "/prijava", http.StatusSeeOther)
		return
	}
	svezi, err := h.KorisniciRepo.DohvatiPoID(r.Context(), k.ID)
	if err != nil || svezi.TotpTajna == "" {
		middleware.SetFlash(w, r, h.DB, "greska", "Dvostepena verifikacija nije uključena.")
		http.Redirect(w, r, "/admin/profil", http.StatusSeeOther)
		return
	}
	h.generisiIPrikaziKodove(w, r, k, "Generisani su novi rezervni kodovi. Stari više ne važe.")
}

// AdminTotpDeaktivacija uklanja TOTP tajnu
func (h *Handler) AdminTotpDeaktivacija(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if k == nil {
		http.Redirect(w, r, "/prijava", http.StatusSeeOther)
		return
	}

	if err := h.KorisniciRepo.SacuvajTotpTajnu(r.Context(), k.ID, ""); err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju. Pokušajte ponovo.")
		http.Redirect(w, r, "/admin/profil", http.StatusSeeOther)
		return
	}
	// isključenjem 2FA brišemo i rezervne kodove
	_ = h.RezervniKodoviRepo.Obrisi(r.Context(), k.ID)

	middleware.SetFlash(w, r, h.DB, "uspeh", "Dvostepena verifikacija je isključena.")
	http.Redirect(w, r, "/admin/profil", http.StatusSeeOther)
}

// AdminLoginIstorija prikazuje evidenciju prijava za datog korisnika
func (h *Handler) AdminLoginIstorija(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if !middleware.JeAdmin(k) {
		http.Error(w, "Pristup odbijen", http.StatusForbidden)
		return
	}

	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Redirect(w, r, "/admin/korisnici", http.StatusSeeOther)
		return
	}

	korisnik, err := h.KorisniciRepo.DohvatiPoID(r.Context(), id)
	if err != nil {
		http.Error(w, "Korisnik nije pronađen", http.StatusNotFound)
		return
	}

	// admin ne sme da vidi istoriju superadmin naloga
	if korisnik.Uloga == "superadmin" && k.Uloga != "superadmin" {
		http.Error(w, "Pristup odbijen", http.StatusForbidden)
		return
	}

	istorija, err := h.LoginIstorijsaRepo.ListaZaKorisnika(r.Context(), id, 50)
	if err != nil {
		http.Error(w, "Greška pri učitavanju istorije", http.StatusInternalServerError)
		return
	}

	podesavanja, _ := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "admin"
	ps.NaslovStranice = "Istorija prijava — " + korisnik.KorisnickoIme

	h.renderujTemplate(w, "admin_login_istorija", podaciLoginIstorija{
		PodaciStranice: ps,
		PrikazKorisnik: *korisnik,
		Istorija:       istorija,
	})
}

type podaciAdminDozvole struct {
	model.PodaciStranice
	Korisnici         []model.Korisnik
	TrenutniID        int64
	DozvoleRadnik     map[string]bool
	DozvoleAdmin      map[string]bool
	DozvoleSuperadmin map[string]bool
}

// AdminDozvole prikazuje stranicu za upravljanje ulogama i pregled dozvola
func (h *Handler) AdminDozvole(w http.ResponseWriter, r *http.Request) {
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
	ps.Stranica = "dozvole"
	ps.NaslovStranice = "Dozvole"

	dozvoleAdmin := map[string]bool{}
	dozvoleSuperadmin := map[string]bool{}
	if k.Uloga == "superadmin" {
		dozvoleAdmin = h.DozvoleRepo.SveDozvole(r.Context(), "admin")
		dozvoleSuperadmin = h.DozvoleRepo.SveDozvole(r.Context(), "superadmin")
	}

	h.renderujTemplate(w, "admin_dozvole", podaciAdminDozvole{
		PodaciStranice:    ps,
		Korisnici:         lista,
		TrenutniID:        k.ID,
		DozvoleRadnik:     h.DozvoleRepo.SveDozvole(r.Context(), "radnik"),
		DozvoleAdmin:      dozvoleAdmin,
		DozvoleSuperadmin: dozvoleSuperadmin,
	})
}

// AdminDozvolePromeniUlogu menja ulogu korisnika sa stranice dozvola
func (h *Handler) AdminDozvolePromeniUlogu(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if k == nil || k.Uloga != "superadmin" {
		http.Error(w, "Pristup odbijen", http.StatusForbidden)
		return
	}

	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Proverite unete podatke.")
		http.Redirect(w, r, "/admin/dozvole", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Proverite unete podatke.")
		http.Redirect(w, r, "/admin/dozvole", http.StatusSeeOther)
		return
	}

	// superadmin ne može menjati svoju vlastitu ulogu
	if id == k.ID {
		middleware.SetFlash(w, r, h.DB, "greska", "Ova radnja nije dozvoljena.")
		http.Redirect(w, r, "/admin/dozvole", http.StatusSeeOther)
		return
	}

	uloga := r.FormValue("uloga")
	// superadmin uloga se ne može dodeliti kroz interfejs
	validneUloge := map[string]bool{"admin": true, "radnik": true}
	if !validneUloge[uloga] {
		middleware.SetFlash(w, r, h.DB, "greska", "Proverite unete podatke.")
		http.Redirect(w, r, "/admin/dozvole", http.StatusSeeOther)
		return
	}

	ciljni, err := h.KorisniciRepo.DohvatiPoID(r.Context(), id)
	if err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju. Pokušajte ponovo.")
		http.Redirect(w, r, "/admin/dozvole", http.StatusSeeOther)
		return
	}

	// superadmin uloga se ne može menjati
	if ciljni.Uloga == "superadmin" {
		middleware.SetFlash(w, r, h.DB, "greska", "Ova radnja nije dozvoljena.")
		http.Redirect(w, r, "/admin/dozvole", http.StatusSeeOther)
		return
	}

	if err := h.KorisniciRepo.AzurirajUlogu(r.Context(), id, uloga); err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju. Pokušajte ponovo.")
		http.Redirect(w, r, "/admin/dozvole", http.StatusSeeOther)
		return
	}

	middleware.SetFlash(w, r, h.DB, "uspeh", "Promene su uspešno sačuvane.")
	http.Redirect(w, r, "/admin/dozvole", http.StatusSeeOther)
}

// AdminDozvoleSacuvaj prima POST i čuva promene dozvola za radnik i admin uloge
func (h *Handler) AdminDozvoleSacuvaj(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if !middleware.JeAdmin(k) {
		http.Error(w, "Pristup odbijen", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Proverite unete podatke.")
		http.Redirect(w, r, "/admin/dozvole", http.StatusSeeOther)
		return
	}
	// čuvamo dozvole: superadmin menja radnik i admin, admin menja samo radnik
	uloge := []string{"radnik", "admin"}
	if k.Uloga != "superadmin" {
		uloge = []string{"radnik"}
	}
	for _, uloga := range uloge {
		for _, akcija := range middleware.SveAkcije() {
			kljuc := uloga + "__" + akcija
			dozvoljeno := r.FormValue(kljuc) == "on"
			if err := h.DozvoleRepo.Sacuvaj(r.Context(), uloga, akcija, dozvoljeno); err != nil {
				middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju. Pokušajte ponovo.")
				http.Redirect(w, r, "/admin/dozvole", http.StatusSeeOther)
				return
			}
		}
	}
	middleware.SetFlash(w, r, h.DB, "uspeh", "Promene su uspešno sačuvane.")
	http.Redirect(w, r, "/admin/dozvole", http.StatusSeeOther)
}

// AdminDozvoleReset vraća sve dozvole na podrazumevane vrednosti — samo superadmin
func (h *Handler) AdminDozvoleReset(w http.ResponseWriter, r *http.Request) {
	k := middleware.KorisnikIzKonteksta(r.Context())
	if k == nil || k.Uloga != "superadmin" {
		http.Error(w, "Pristup odbijen", http.StatusForbidden)
		return
	}
	if err := h.DozvoleRepo.Reset(r.Context()); err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Greška pri čuvanju. Pokušajte ponovo.")
		http.Redirect(w, r, "/admin/dozvole", http.StatusSeeOther)
		return
	}
	middleware.SetFlash(w, r, h.DB, "uspeh", "Dozvole su vraćene na podrazumevane vrednosti.")
	http.Redirect(w, r, "/admin/dozvole", http.StatusSeeOther)
}
