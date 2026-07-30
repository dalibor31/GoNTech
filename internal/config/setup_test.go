package config

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestObradiSetupPotvrdiNevazeciJsonOdbija(t *testing.T) {
	envFajl := filepath.Join(t.TempDir(), "ntech.env")
	gotov := make(chan struct{})

	req := httptest.NewRequest("POST", "/setup/potvrdi", strings.NewReader("nije json"))
	w := httptest.NewRecorder()
	obradiSetupPotvrdi(envFajl, gotov)(w, req)

	if w.Code != 400 {
		t.Fatalf("status = %d, očekivano 400 za nevažeći JSON", w.Code)
	}
	if _, err := os.Stat(envFajl); err == nil {
		t.Fatal("ntech.env ne sme biti upisan kad je JSON nevažeći")
	}
	select {
	case <-gotov:
		t.Fatal("gotov kanal ne sme biti zatvoren kad zahtev nije uspeo")
	default:
	}
}

func TestObradiSetupPotvrdiPortNulaOdbija(t *testing.T) {
	envFajl := filepath.Join(t.TempDir(), "ntech.env")
	gotov := make(chan struct{})

	req := httptest.NewRequest("POST", "/setup/potvrdi", strings.NewReader(`{"port":0}`))
	w := httptest.NewRecorder()
	obradiSetupPotvrdi(envFajl, gotov)(w, req)

	if w.Code != 400 {
		t.Fatalf("status = %d, očekivano 400 za port 0", w.Code)
	}
}

func TestObradiSetupPotvrdiPortVanOpsegaOdbija(t *testing.T) {
	envFajl := filepath.Join(t.TempDir(), "ntech.env")
	gotov := make(chan struct{})

	req := httptest.NewRequest("POST", "/setup/potvrdi", strings.NewReader(`{"port":99999}`))
	w := httptest.NewRecorder()
	obradiSetupPotvrdi(envFajl, gotov)(w, req)

	if w.Code != 400 {
		t.Fatalf("status = %d, očekivano 400 za port van opsega", w.Code)
	}
}

func TestObradiSetupPotvrdiValidanPortUpisuje(t *testing.T) {
	envFajl := filepath.Join(t.TempDir(), "ntech.env")
	gotov := make(chan struct{})

	req := httptest.NewRequest("POST", "/setup/potvrdi", strings.NewReader(`{"port":8080}`))
	w := httptest.NewRecorder()
	obradiSetupPotvrdi(envFajl, gotov)(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, očekivano 200 za validan port", w.Code)
	}
	sadrzaj, err := os.ReadFile(envFajl)
	if err != nil {
		t.Fatalf("ntech.env nije upisan: %v", err)
	}
	if !strings.Contains(string(sadrzaj), "NTECH_PORT=8080") {
		t.Errorf("ntech.env sadržaj = %q, očekivano NTECH_PORT=8080", sadrzaj)
	}
	select {
	case <-gotov:
	default:
		t.Fatal("gotov kanal mora biti zatvoren posle uspešnog zahteva")
	}
}
