package middleware

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"os"
	"strings"
)

const csrfKolacic = "ntech_csrf"

// maxTeloUpload je gornja granica veličine tela multipart zahteva (upload fajlova).
// Postavlja se u middleware-u pre parsiranja jer čitanje _csrf polja već parsira
// telo — bez ovog limita pojedinačni handleri ne mogu da ga efektivno ograniče.
// Najveći legitiman upload je 5 MB (avatar, pozadina); ostatak je rezerva za form overhead.
const maxTeloUpload = 6 << 20

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
					Secure:   os.Getenv("NTECH_ENV") == "production" || os.Getenv("NTECH_ENV") == "demo",
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
			// ograničavamo veličinu tela PRE parsiranja — čitanje _csrf polja parsira
			// celo telo, pa je ovo jedino mesto gde limit za upload stvarno deluje
			if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
				r.Body = http.MaxBytesReader(w, r.Body, maxTeloUpload)
			}
			// čitamo token iz tela forme ili zaglavlja (za AJAX)
			submitted := r.FormValue("_csrf")
			if submitted == "" {
				submitted = r.Header.Get("X-CSRF-Token")
			}
			if token == "" || subtle.ConstantTimeCompare([]byte(submitted), []byte(token)) != 1 {
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
