package middleware

import "net/http"

// BezbednostHeaders dodaje standardne HTTP security headere na svaki odgovor
func BezbednostHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Frame-Options", "DENY")
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-XSS-Protection", "1; mode=block")
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			h.Set("Content-Security-Policy",
				"default-src 'self'; "+
					"style-src 'self' 'unsafe-inline' https://cdn.tailwindcss.com; "+
				"script-src 'self' 'unsafe-inline' https://cdn.tailwindcss.com https://cdn.jsdelivr.net; "+
					"img-src 'self' data: blob:; "+
					"font-src 'self'; "+
					"connect-src 'self'")
			next.ServeHTTP(w, r)
		})
	}
}
