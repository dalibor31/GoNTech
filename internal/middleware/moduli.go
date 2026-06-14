package middleware

import (
	"context"
	"net/http"
)

// RequireModul je chi middleware koji propušta zahtev samo ako je traženi zakonski
// modul uključen za firmu (prema profilu firme iz podešavanja). Ovo je sloj IZNAD
// RBAC-a: „da li firma uopšte koristi modul", nezavisno od „da li korisnik sme"
// (RequireDozvola). Zahtev mora proći oba sloja.
//
// Provera se prosleđuje kao funkcija (proveri) da paket middleware ne zavisi od
// config/sqlite — isti obrazac kao RequireDozvola. U praksi je to closure koja
// učita podešavanja i pozove config.ModulUkljucen.
func RequireModul(proveri func(ctx context.Context, modul string) bool, modul string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !proveri(r.Context(), modul) {
				postaviFlashGresku(w, "Ovaj modul nije uključen za vašu firmu.")
				http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
