package handler

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"ntech/internal/auth"
	"ntech/internal/middleware"
)

const imeKolacica = "ntech_sesija"
const trajanjeSeije = 7 * 24 * time.Hour
const trajanjePredSeije = 5 * time.Minute

const maxNeuspehaPrijave = 5
const prozorPrijave = 15 * time.Minute
const trajanjeZakljucavanja = 30 * time.Minute

// PrikazPrijave renderuje formu za prijavu
func (h *Handler) PrikazPrijave(w http.ResponseWriter, r *http.Request) {
	postoji, err := h.KorisniciRepo.PostojiIjedan(r.Context())
	if err == nil && !postoji {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}

	greska := r.URL.Query().Get("greska")
	h.renderujStandalone(w, "prijava", map[string]any{
		"Greska":    greska,
		"CsrfToken": middleware.CsrfToken(r.Context()),
	})
}

// Prijava obrađuje POST formu za prijavu
func (h *Handler) Prijava(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/prijava?greska=1", http.StatusSeeOther)
		return
	}

	ip := izvuciIP(r.RemoteAddr)
	korisnickoIme := r.FormValue("korisnicko_ime")
	lozinka := r.FormValue("lozinka")

	// bruteforce provera — blokira IP posle maxNeuspehaPrijave neuspelih pokušaja
	od := time.Now().Add(-prozorPrijave)
	n, _ := h.PokusajiRepo.BrojNeuspeha(r.Context(), ip, od)
	if n >= maxNeuspehaPrijave {
		if preostalo, zaklj := h.preostaloBruteforce(r.Context(), ip, od); zaklj {
			auth.LogZaklucano(ip, korisnickoIme)
			h.renderujStandalone(w, "prijava", map[string]any{
				"Greska":    "zakljucano",
				"Preostalo": preostalo,
				"CsrfToken": middleware.CsrfToken(r.Context()),
			})
			return
		}
		// zaključavanje je isteklo, nastavljamo normalnu obradu
	}

	korisnik, err := h.KorisniciRepo.DohvatiPoImenu(r.Context(), korisnickoIme)
	if err != nil || !korisnik.Aktivan {
		_ = h.PokusajiRepo.Zabeleži(r.Context(), ip, korisnickoIme, false)
		auth.LogNeuspehPrijave(ip, korisnickoIme, "wrong_user")
		http.Redirect(w, r, "/prijava?greska=1", http.StatusSeeOther)
		return
	}

	if !auth.ProveriLozinku(korisnik.LozinkaHash, lozinka) {
		_ = h.PokusajiRepo.Zabeleži(r.Context(), ip, korisnickoIme, false)
		auth.LogNeuspehPrijave(ip, korisnickoIme, "wrong_password")
		http.Redirect(w, r, "/prijava?greska=1", http.StatusSeeOther)
		return
	}

	_ = h.PokusajiRepo.Zabeleži(r.Context(), ip, korisnickoIme, true)
	auth.LogUspehPrijave(ip, korisnickoIme)

	token := auth.GenerisiToken()

	if korisnik.TotpTajna != "" {
		if err := h.SesijeRepo.Kreiraj(r.Context(), korisnik.ID, token, time.Now().Add(trajanjePredSeije), false); err != nil {
			http.Redirect(w, r, "/prijava?greska=2", http.StatusSeeOther)
			return
		}
		http.SetCookie(w, napraviKolacic(token, time.Now().Add(trajanjePredSeije)))
		http.Redirect(w, r, "/prijava/totp", http.StatusSeeOther)
		return
	}

	if err := h.SesijeRepo.Kreiraj(r.Context(), korisnik.ID, token, time.Now().Add(trajanjeSeije), true); err != nil {
		http.Redirect(w, r, "/prijava?greska=2", http.StatusSeeOther)
		return
	}
	http.SetCookie(w, napraviKolacic(token, time.Now().Add(trajanjeSeije)))
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

// PrikazTotp renderuje formu za unos TOTP koda
func (h *Handler) PrikazTotp(w http.ResponseWriter, r *http.Request) {
	kolacic, err := r.Cookie(imeKolacica)
	if err != nil {
		http.Redirect(w, r, "/prijava", http.StatusSeeOther)
		return
	}
	sesija, err := h.SesijeRepo.DohvatiPoTokenu(r.Context(), kolacic.Value)
	if err != nil || sesija.TotpPotvrdjeno || time.Now().After(sesija.DatumIsteka) {
		http.Redirect(w, r, "/prijava", http.StatusSeeOther)
		return
	}

	greska := r.URL.Query().Get("greska")
	h.renderujStandalone(w, "totp_provera", map[string]any{
		"Greska":    greska,
		"CsrfToken": middleware.CsrfToken(r.Context()),
	})
}

// VerifikujTotp obrađuje POST formu za TOTP verifikaciju
func (h *Handler) VerifikujTotp(w http.ResponseWriter, r *http.Request) {
	kolacic, err := r.Cookie(imeKolacica)
	if err != nil {
		http.Redirect(w, r, "/prijava", http.StatusSeeOther)
		return
	}

	sesija, err := h.SesijeRepo.DohvatiPoTokenu(r.Context(), kolacic.Value)
	if err != nil || sesija.TotpPotvrdjeno || time.Now().After(sesija.DatumIsteka) {
		http.Redirect(w, r, "/prijava", http.StatusSeeOther)
		return
	}

	korisnik, err := h.KorisniciRepo.DohvatiPoID(r.Context(), sesija.KorisnikID)
	if err != nil {
		http.Redirect(w, r, "/prijava", http.StatusSeeOther)
		return
	}

	kod := r.FormValue("kod")
	if !auth.VerifikujTotpKod(kod, korisnik.TotpTajna) {
		http.Redirect(w, r, "/prijava/totp?greska=1", http.StatusSeeOther)
		return
	}

	novoIstice := time.Now().Add(trajanjeSeije)
	if err := h.SesijeRepo.PotvrdiTotp(r.Context(), kolacic.Value, novoIstice); err != nil {
		http.Redirect(w, r, "/prijava?greska=2", http.StatusSeeOther)
		return
	}

	http.SetCookie(w, napraviKolacic(kolacic.Value, novoIstice))
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

// PrikazSetupa renderuje setup wizard za prvog korisnika
func (h *Handler) PrikazSetupa(w http.ResponseWriter, r *http.Request) {
	postoji, err := h.KorisniciRepo.PostojiIjedan(r.Context())
	if err == nil && postoji {
		http.Redirect(w, r, "/prijava", http.StatusSeeOther)
		return
	}
	greska := r.URL.Query().Get("greska")
	h.renderujStandalone(w, "setup", map[string]any{
		"Greska":    greska,
		"CsrfToken": middleware.CsrfToken(r.Context()),
	})
}

// SacuvajSetup kreira prvog superadmin korisnika
func (h *Handler) SacuvajSetup(w http.ResponseWriter, r *http.Request) {
	postoji, err := h.KorisniciRepo.PostojiIjedan(r.Context())
	if err == nil && postoji {
		http.Redirect(w, r, "/prijava", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/setup?greska=1", http.StatusSeeOther)
		return
	}

	ime := r.FormValue("korisnicko_ime")
	lozinka := r.FormValue("lozinka")
	potvrda := r.FormValue("lozinka_potvrda")

	if len(ime) < 3 || len(lozinka) < 8 || lozinka != potvrda {
		http.Redirect(w, r, "/setup?greska=1", http.StatusSeeOther)
		return
	}

	hash, err := auth.HashujLozinku(lozinka)
	if err != nil {
		http.Redirect(w, r, "/setup?greska=2", http.StatusSeeOther)
		return
	}

	if _, err := h.KorisniciRepo.Kreiraj(r.Context(), ime, hash, "superadmin"); err != nil {
		http.Redirect(w, r, "/setup?greska=2", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/prijava?sacuvano=1", http.StatusSeeOther)
}

// Odjava briše sesiju i kolačić
func (h *Handler) Odjava(w http.ResponseWriter, r *http.Request) {
	if kolacic, err := r.Cookie(imeKolacica); err == nil {
		_ = h.SesijeRepo.Obrisi(r.Context(), kolacic.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:    imeKolacica,
		Value:   "",
		Path:    "/",
		Expires: time.Unix(0, 0),
		MaxAge:  -1,
	})
	http.Redirect(w, r, "/prijava", http.StatusSeeOther)
}

// preostaloBruteforce vraća formatiranu poruku o preostatom vremenu zaključavanja i bool da li je IP još zaključan
func (h *Handler) preostaloBruteforce(ctx context.Context, ip string, od time.Time) (string, bool) {
	vreme, ok, _ := h.PokusajiRepo.VremePoslednjeg(ctx, ip, od)
	if !ok {
		return "", false
	}
	d := time.Until(vreme.Add(trajanjeZakljucavanja)).Round(time.Second)
	if d <= 0 {
		return "", false
	}
	min := int(d.Minutes())
	sek := int(d.Seconds()) % 60
	if min > 0 {
		return fmt.Sprintf("%d min %d sek", min, sek), true
	}
	return fmt.Sprintf("%d sek", sek), true
}

// izvuciIP izdvaja IP adresu iz RemoteAddr formata "ip:port"
func izvuciIP(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}

func napraviKolacic(token string, istice time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     imeKolacica,
		Value:    token,
		Path:     "/",
		Expires:  istice,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}
}
