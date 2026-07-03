package handler

import (
	"net/http"
	"testing"
)

func TestIzvuciIP(t *testing.T) {
	testovi := []struct {
		naziv      string
		realIP     string
		forwarded  string
		remoteAddr string
		ocek       string
	}{
		{"X-Real-IP ima prioritet kad je proxy poverljiv (loopback)", "1.2.3.4", "9.9.9.9", "127.0.0.1:1234", "1.2.3.4"},
		{"X-Real-IP ima prioritet kad je proxy poverljiv (privatna Docker mreža)", "1.2.3.4", "9.9.9.9", "172.18.0.1:1234", "1.2.3.4"},
		{"poslednji X-Forwarded-For kad je proxy poverljiv", "", "1.1.1.1, 2.2.2.2, 3.3.3.3", "127.0.0.1:1234", "3.3.3.3"},
		{"zaglavlje se IGNORIŠE kad RemoteAddr nije poverljiv (javna adresa — spoofing)", "1.2.3.4", "", "5.5.5.5:1234", "5.5.5.5"},
		{"RemoteAddr bez porta kad nema zaglavlja", "", "", "5.5.5.5:1234", "5.5.5.5"},
		{"RemoteAddr kakav jeste ako nije host:port", "", "", "neispravan", "neispravan"},
	}
	for _, tt := range testovi {
		t.Run(tt.naziv, func(t *testing.T) {
			r, _ := http.NewRequest("GET", "/", nil)
			r.RemoteAddr = tt.remoteAddr
			if tt.realIP != "" {
				r.Header.Set("X-Real-IP", tt.realIP)
			}
			if tt.forwarded != "" {
				r.Header.Set("X-Forwarded-For", tt.forwarded)
			}
			if got := izvuciIP(r); got != tt.ocek {
				t.Errorf("izvuciIP() = %q, očekivano %q", got, tt.ocek)
			}
		})
	}
}

func TestParseOpcionuCenu(t *testing.T) {
	if got := parseOpcionuCenu("  "); got != nil {
		t.Errorf("prazan unos → nil, dobijeno %v", *got)
	}
	if got := parseOpcionuCenu("abc"); got != nil {
		t.Errorf("neispravan broj → nil, dobijeno %v", *got)
	}
	if got := parseOpcionuCenu("-5"); got != nil {
		t.Errorf("negativan broj → nil, dobijeno %v", *got)
	}
	if got := parseOpcionuCenu("1250.50"); got == nil || *got != 1250.50 {
		t.Errorf("ispravan broj → 1250.50, dobijeno %v", got)
	}
}

func TestValidnoImeBackupa(t *testing.T) {
	ispravna := []string{"ntech_20260612_153000.db"}
	for _, ime := range ispravna {
		if !validnoImeBackupa.MatchString(ime) {
			t.Errorf("%q bi trebalo da je ispravno", ime)
		}
	}
	// anti path-traversal i neispravni formati
	neispravna := []string{
		"../ntech_20260612_153000.db",
		"ntech_20260612_153000.db.bak",
		"ntech_2026_153000.db",
		"ntech_20260612_153000.sqlite",
		"/etc/passwd",
		"ntech_20260612_153000.db/../x",
	}
	for _, ime := range neispravna {
		if validnoImeBackupa.MatchString(ime) {
			t.Errorf("%q NE bi smelo da prođe", ime)
		}
	}
}
