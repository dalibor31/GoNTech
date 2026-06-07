package middleware

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"os"
)

const csrfKolacic = "ntech_csrf"

type csrfKljucTip struct{}

var csrfKljuc = csrfKljucTip{}

// CsrfToken vraća CSRF token iz konteksta zahteva (postavlja ga CsrfMiddleware)
func CsrfToken(ctx context.Context) string {
	if v, ok := ctx.Value(csrfKljuc).(string); ok {
		return v
	}
	return ""
}

// CsrfMiddleware generiše i validira CSRF tokene metodom cookie + skriveno polje
func CsrfMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// čita postojeći ili generiše novi token
		token := ""
		if k, err := r.Cookie(csrfKolacic); err == nil && k.Value != "" {
			token = k.Value
		} else {
			b := make([]byte, 32)
			if _, err := rand.Read(b); err == nil {
				token = base64.RawURLEncoding.EncodeToString(b)
				http.SetCookie(w, &http.Cookie{
					Name:     csrfKolacic,
					Value:    token,
					Path:     "/",
					MaxAge:   86400 * 30,
					HttpOnly: true,
					Secure:   os.Getenv("NTECH_ENV") == "production",
					SameSite: http.SameSiteStrictMode,
				})
			}
		}

		// ubacujemo token u context radi dostupnosti u handlerima i šablonima
		ctx := context.WithValue(r.Context(), csrfKljuc, token)
		r = r.WithContext(ctx)

		// validiramo na svim mutabilnim HTTP metodama
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			// čitamo token iz tela forme ili zaglavlja (za AJAX)
			submitted := r.FormValue("_csrf")
			if submitted == "" {
				submitted = r.Header.Get("X-CSRF-Token")
			}
			if token == "" || submitted != token {
				http.Error(w,
					"Neispravan sigurnosni token. Osvežite stranicu i pokušajte ponovo.",
					http.StatusForbidden,
				)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
