package handler

import (
	"context"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ntech/internal/auth"
	"ntech/internal/db/sqlite"

	"github.com/pquerna/otp/totp"
)

const testIP = "1.2.3.4"

// testHandler pravi Handler nad privremenom bazom sa pravim migracijama.
// TemplatesFS je koren repo-a (../..) da standalone render (zakljucano) radi.
func testHandler(t *testing.T) *Handler {
	t.Helper()
	putanja := filepath.Join(t.TempDir(), "test.db")
	db, err := sqlite.OtvoriDB(putanja)
	if err != nil {
		t.Fatalf("OtvoriDB: %v", err)
	}
	if err := sqlite.PokreniMigracije(db, os.DirFS("../..")); err != nil {
		t.Fatalf("migracije: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	kljuc := make([]byte, 32)
	rand.Read(kljuc)
	h := Novi(db, kljuc)
	h.TemplatesFS = os.DirFS("../..")
	return h
}

func seedKorisnik(t *testing.T, h *Handler, ime, lozinka, uloga string) int64 {
	t.Helper()
	hash, err := auth.HashujLozinku(lozinka)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	k, err := h.KorisniciRepo.Kreiraj(context.Background(), ime, hash, uloga)
	if err != nil {
		t.Fatalf("Kreiraj korisnika: %v", err)
	}
	return k.ID
}

// postPrijava gradi POST /prijava zahtev sa datim kredencijalima
func postPrijava(ime, lozinka string) *http.Request {
	form := url.Values{}
	form.Set("korisnicko_ime", ime)
	form.Set("lozinka", lozinka)
	r := httptest.NewRequest("POST", "/prijava", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.RemoteAddr = testIP + ":5555"
	return r
}

// sesijskiKolacic vraća vrednost kolačića ntech_sesija iz odgovora, ili "" ako ga nema
func sesijskiKolacic(res *http.Response) string {
	for _, c := range res.Cookies() {
		if c.Name == imeKolacica && c.MaxAge >= 0 && c.Value != "" {
			return c.Value
		}
	}
	return ""
}

func TestPrijavaUspeh(t *testing.T) {
	h := testHandler(t)
	seedKorisnik(t, h, "pera", "tajna123", "radnik")

	w := httptest.NewRecorder()
	h.Prijava(w, postPrijava("pera", "tajna123"))
	res := w.Result()

	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("status = %d, očekivano 303", res.StatusCode)
	}
	if loc := res.Header.Get("Location"); loc != "/dashboard" {
		t.Fatalf("Location = %q, očekivano /dashboard", loc)
	}
	if sesijskiKolacic(res) == "" {
		t.Fatal("nije postavljen sesijski kolačić")
	}
}

func TestPrijavaPogresnaLozinka(t *testing.T) {
	h := testHandler(t)
	seedKorisnik(t, h, "pera", "tajna123", "radnik")

	w := httptest.NewRecorder()
	h.Prijava(w, postPrijava("pera", "pogresna"))
	res := w.Result()

	if loc := res.Header.Get("Location"); loc != "/prijava?greska=1" {
		t.Fatalf("Location = %q, očekivano /prijava?greska=1", loc)
	}
	if sesijskiKolacic(res) != "" {
		t.Fatal("sesijski kolačić NE bi smeo da postoji uz pogrešnu lozinku")
	}
	// neuspeh je zabeležen za brute-force
	n, _ := h.PokusajiRepo.BrojNeuspeha(context.Background(), testIP, time.Now().Add(-time.Minute))
	if n != 1 {
		t.Fatalf("zabeleženo %d neuspeha, očekivano 1", n)
	}
}

func TestPrijavaNepostojeciKorisnik(t *testing.T) {
	h := testHandler(t)
	w := httptest.NewRecorder()
	h.Prijava(w, postPrijava("nema_me", "bilosta"))
	res := w.Result()

	if loc := res.Header.Get("Location"); loc != "/prijava?greska=1" {
		t.Fatalf("Location = %q, očekivano /prijava?greska=1", loc)
	}
	if sesijskiKolacic(res) != "" {
		t.Fatal("sesijski kolačić NE bi smeo da postoji")
	}
}

func TestPrijavaNeaktivanNalog(t *testing.T) {
	h := testHandler(t)
	id := seedKorisnik(t, h, "pera", "tajna123", "radnik")
	if err := h.KorisniciRepo.AzurirajAktivan(context.Background(), id, false); err != nil {
		t.Fatalf("deaktivacija: %v", err)
	}

	w := httptest.NewRecorder()
	h.Prijava(w, postPrijava("pera", "tajna123")) // ispravna lozinka, ali neaktivan
	res := w.Result()

	if loc := res.Header.Get("Location"); loc != "/prijava?greska=1" {
		t.Fatalf("Location = %q, očekivano /prijava?greska=1", loc)
	}
	if sesijskiKolacic(res) != "" {
		t.Fatal("neaktivan nalog NE sme dobiti sesiju")
	}
}

func TestPrijavaZakljucavanjeIP(t *testing.T) {
	h := testHandler(t)
	seedKorisnik(t, h, "pera", "tajna123", "radnik")

	// 5 neuspelih pokušaja sa istog IP-a
	for i := 0; i < maxNeuspehaPrijave; i++ {
		w := httptest.NewRecorder()
		h.Prijava(w, postPrijava("pera", "pogresna"))
	}

	// 6. pokušaj sa ISPRAVNOM lozinkom mora biti zaključan
	w := httptest.NewRecorder()
	h.Prijava(w, postPrijava("pera", "tajna123"))
	res := w.Result()

	if sesijskiKolacic(res) != "" {
		t.Fatal("zaključan IP NE sme dobiti sesiju ni uz ispravnu lozinku")
	}
	if loc := res.Header.Get("Location"); loc == "/dashboard" {
		t.Fatal("zaključan IP je preusmeren na /dashboard")
	}
}

func TestTotpTok(t *testing.T) {
	h := testHandler(t)
	ctx := context.Background()
	id := seedKorisnik(t, h, "pera", "tajna123", "radnik")

	const tajna = "JBSWY3DPEHPK3PXP"
	if err := h.KorisniciRepo.SacuvajTotpTajnu(ctx, id, tajna); err != nil {
		t.Fatalf("SacuvajTotpTajnu: %v", err)
	}

	// prijava lozinkom → pred-sesija + redirect na /prijava/totp (NE /dashboard)
	w := httptest.NewRecorder()
	h.Prijava(w, postPrijava("pera", "tajna123"))
	res := w.Result()
	if loc := res.Header.Get("Location"); loc != "/prijava/totp" {
		t.Fatalf("Location = %q, očekivano /prijava/totp", loc)
	}
	token := sesijskiKolacic(res)
	if token == "" {
		t.Fatal("nije postavljen pred-sesijski kolačić")
	}

	// generiši validan TOTP kod i potvrdi
	kod, err := totp.GenerateCode(tajna, time.Now())
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	form := url.Values{}
	form.Set("kod", kod)
	r2 := httptest.NewRequest("POST", "/prijava/totp", strings.NewReader(form.Encode()))
	r2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r2.AddCookie(&http.Cookie{Name: imeKolacica, Value: token})
	w2 := httptest.NewRecorder()
	h.VerifikujTotp(w2, r2)
	res2 := w2.Result()
	if loc := res2.Header.Get("Location"); loc != "/dashboard" {
		t.Fatalf("posle TOTP-a Location = %q, očekivano /dashboard", loc)
	}

	// sesija je sada potvrđena
	ses, err := h.SesijeRepo.DohvatiPoTokenu(ctx, token)
	if err != nil {
		t.Fatalf("DohvatiPoTokenu: %v", err)
	}
	if !ses.TotpPotvrdjeno {
		t.Fatal("sesija nije označena kao TOTP-potvrđena")
	}
}

func TestTotpPogresanKod(t *testing.T) {
	h := testHandler(t)
	ctx := context.Background()
	id := seedKorisnik(t, h, "pera", "tajna123", "radnik")
	_ = h.KorisniciRepo.SacuvajTotpTajnu(ctx, id, "JBSWY3DPEHPK3PXP")

	w := httptest.NewRecorder()
	h.Prijava(w, postPrijava("pera", "tajna123"))
	token := sesijskiKolacic(w.Result())

	form := url.Values{}
	form.Set("kod", "000000")
	r := httptest.NewRequest("POST", "/prijava/totp", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(&http.Cookie{Name: imeKolacica, Value: token})
	w2 := httptest.NewRecorder()
	h.VerifikujTotp(w2, r)

	if loc := w2.Result().Header.Get("Location"); loc != "/prijava/totp?greska=1" {
		t.Fatalf("Location = %q, očekivano /prijava/totp?greska=1", loc)
	}
	// sesija ostaje nepotvrđena
	if ses, _ := h.SesijeRepo.DohvatiPoTokenu(ctx, token); ses != nil && ses.TotpPotvrdjeno {
		t.Fatal("sesija ne sme biti potvrđena uz pogrešan kod")
	}
}
