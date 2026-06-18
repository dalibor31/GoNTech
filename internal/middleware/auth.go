package middleware

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"os"
	"time"

	"ntech/internal/db/sqlite"
	"ntech/internal/model"
)

type kontekstKljuc string

// KljucKorisnika je ključ za korisnika u request contextu
const KljucKorisnika kontekstKljuc = "korisnik"

// RequireAuth je chi middleware koji proverava sesiju i injektuje korisnika u context.
// totpKljuc je ključ za dešifrovanje TOTP tajne (korisRepo ga koristi pri čitanju).
func RequireAuth(db *sql.DB, totpKljuc []byte) func(http.Handler) http.Handler {
	sesijeRepo := sqlite.NoviSesijeRepo(db)
	korisRepo := sqlite.NoviKorisniciRepo(db, totpKljuc)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			kolacic, err := r.Cookie("ntech_sesija")
			if err != nil {
				http.Redirect(w, r, "/prijava", http.StatusSeeOther)
				return
			}

			sesija, err := sesijeRepo.DohvatiPoTokenu(r.Context(), kolacic.Value)
			if err != nil {
				// nevažeći token — briši kolačić i preusmeri
				http.SetCookie(w, &http.Cookie{
					Name:     "ntech_sesija",
					Value:    "",
					Path:     "/",
					Expires:  time.Unix(0, 0),
					MaxAge:   -1,
					Secure:   os.Getenv("NTECH_ENV") == "production" || os.Getenv("NTECH_ENV") == "demo",
					HttpOnly: true,
				})
				http.Redirect(w, r, "/prijava", http.StatusSeeOther)
				return
			}

			// proveri da li je sesija istekla
			if time.Now().After(sesija.DatumIsteka) {
				_ = sesijeRepo.Obrisi(r.Context(), kolacic.Value)
				http.Redirect(w, r, "/prijava", http.StatusSeeOther)
				return
			}

			// ako čeka TOTP verifikaciju, preusmeri na TOTP stranicu
			if !sesija.TotpPotvrdjeno {
				http.Redirect(w, r, "/prijava/totp", http.StatusSeeOther)
				return
			}

			korisnik, err := korisRepo.DohvatiPoID(r.Context(), sesija.KorisnikID)
			if err != nil || !korisnik.Aktivan {
				_ = sesijeRepo.Obrisi(r.Context(), kolacic.Value)
				http.Redirect(w, r, "/prijava", http.StatusSeeOther)
				return
			}

			ctx := context.WithValue(r.Context(), KljucKorisnika, korisnik)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// KorisnikIzKonteksta vraća trenutno prijavljenog korisnika iz konteksta
func KorisnikIzKonteksta(ctx context.Context) *model.Korisnik {
	k, _ := ctx.Value(KljucKorisnika).(*model.Korisnik)
	return k
}

// JeAdmin proverava da li korisnik ima admin ili superadmin ulogu
func JeAdmin(k *model.Korisnik) bool {
	if k == nil {
		return false
	}
	return k.Uloga == "admin" || k.Uloga == "superadmin"
}

// RequireSuperAdmin je middleware koji propušta samo superadmin korisnike
func RequireSuperAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		k := KorisnikIzKonteksta(r.Context())
		if k == nil || k.Uloga != "superadmin" {
			postaviFlashGresku(w, "Nemate dozvolu za ovu stranicu.")
			http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireAdmin je middleware koji propušta admin i superadmin korisnike
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		k := KorisnikIzKonteksta(r.Context())
		if k == nil || (k.Uloga != "admin" && k.Uloga != "superadmin") {
			postaviFlashGresku(w, "Nemate dozvolu za ovu stranicu.")
			http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireDozvola je middleware koji propušta korisnika samo ako njegova uloga
// ima traženu akciju. Provera je DB-backed (prosleđuje se DozvoleRepo.ImaDozvolu),
// tako da poštuje izmene matrice dozvola. Koristi se na nivou rute za "pregled"
// stranica, umesto ponavljanja iste provere u svakom handleru.
func RequireDozvola(proveri func(ctx context.Context, uloga, akcija string) bool, akcija string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			k := KorisnikIzKonteksta(r.Context())
			if k == nil || !proveri(r.Context(), k.Uloga, akcija) {
				postaviFlashGresku(w, "Nemate dozvolu za ovu stranicu.")
				http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireDozvolaMut je kao RequireDozvola, ali namenjen mutirajućim rutama
// (POST i akcije brisanja). Na odbijanje vraća 403 sa srpskom porukom umesto
// redirecta — isto kao handler.zahtevajDozvolu, pa ne kvari AJAX/fetch pozive.
// Postavljanjem ove provere na ruteru, zaštita mutacije više ne zavisi od toga
// da li je programer dodao proveru u samom handleru.
func RequireDozvolaMut(proveri func(ctx context.Context, uloga, akcija string) bool, akcija string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			k := KorisnikIzKonteksta(r.Context())
			if k == nil || !proveri(r.Context(), k.Uloga, akcija) {
				http.Error(w, "Nemate dozvolu za ovu akciju.", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// postaviFlashGresku upisuje jednokratnu poruku o grešci u kolačić
func postaviFlashGresku(w http.ResponseWriter, poruka string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "ntech_flash_greska",
		Value:    poruka,
		Path:     "/",
		MaxAge:   60,
		HttpOnly: true,
		Secure:   os.Getenv("NTECH_ENV") == "production" || os.Getenv("NTECH_ENV") == "demo",
	})
}

// ErrNijePrijavljen se vraća kada korisnik nije u contextu
var ErrNijePrijavljen = errors.New("korisnik nije prijavljen")
