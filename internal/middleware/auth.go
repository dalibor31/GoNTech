package middleware

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"ntech/internal/db/sqlite"
	"ntech/internal/model"
)

type kontekstKljuc string

// KljucKorisnika je ključ za korisnika u request contextu
const KljucKorisnika kontekstKljuc = "korisnik"

// RequireAuth je chi middleware koji proverava sesiju i injektuje korisnika u context
func RequireAuth(db *sql.DB) func(http.Handler) http.Handler {
	sesijeRepo := sqlite.NoviSesijeRepo(db)
	korisRepo := sqlite.NoviKorisniciRepo(db)

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
					Name:    "ntech_sesija",
					Value:   "",
					Path:    "/",
					Expires: time.Unix(0, 0),
					MaxAge:  -1,
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

// ErrNijePrijavljen se vraća kada korisnik nije u contextu
var ErrNijePrijavljen = errors.New("korisnik nije prijavljen")
