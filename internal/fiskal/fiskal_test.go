package fiskal

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testniZahtev() InvoiceRequest {
	return InvoiceRequest{
		InvoiceRequest: InvoiceRequestBody{
			InvoiceType:     "Normal",
			TransactionType: "Sale",
			Payment:         []PaymentItem{{Amount: 100, PaymentType: "Cash"}},
			Items:           []InvoiceItem{{Name: "Test", TotalAmount: 100, UnitPrice: 100, Quantity: 1}},
			Cashier:         "Test Kasir",
		},
	}
}

func TestIzdajRacunUspeh(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/invoices" {
			t.Fatalf("neočekivan zahtev: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization header = %q, očekivano Bearer test-key", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"invoiceNumber":"ПП123","totalAmount":100,"totalTax":16.67}`))
	}))
	defer srv.Close()

	k := NoviKlijent(srv.URL, "test-key")
	odgovor, err := k.IzdajRacun(context.Background(), testniZahtev())
	if err != nil {
		t.Fatalf("IzdajRacun greška: %v", err)
	}
	if odgovor.InvoiceNumber != "ПП123" {
		t.Errorf("InvoiceNumber = %q, očekivano ПП123", odgovor.InvoiceNumber)
	}
	if odgovor.TotalAmount != 100 {
		t.Errorf("TotalAmount = %v, očekivano 100", odgovor.TotalAmount)
	}
}

func TestIzdajRacunHttpGreska(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"message":"nevažeći zahtev"}`))
	}))
	defer srv.Close()

	k := NoviKlijent(srv.URL, "test-key")
	if _, err := k.IzdajRacun(context.Background(), testniZahtev()); err == nil {
		t.Fatal("očekivana greška za HTTP 400, dobijeno nil")
	}
}

func TestIzdajRacunServerNedostupan(t *testing.T) {
	// port na kome sigurno niko ne sluša (zatvoren odmah posle otvaranja)
	ln := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := ln.URL
	ln.Close()

	k := NoviKlijent(url, "test-key")
	if _, err := k.IzdajRacun(context.Background(), testniZahtev()); err == nil {
		t.Fatal("očekivana greška kad server nije dostupan, dobijeno nil")
	}
}

func TestIzdajRacunKontekstIstekao(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	k := NoviKlijent(srv.URL, "test-key")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	if _, err := k.IzdajRacun(ctx, testniZahtev()); err == nil {
		t.Fatal("očekivana greška kad kontekst istekne pre odgovora, dobijeno nil")
	}
}

func TestIzdajRacunPrezivljavaOtkazivanjeOriginalnogKonteksta(t *testing.T) {
	// simulira: HTTP handler-ov r.Context() se otkaže (npr. klijent
	// zatvori konekciju), ali fiskalni poziv nastavlja preko odvojenog konteksta.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(30 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"invoiceNumber":"ПП999"}`))
	}))
	defer srv.Close()

	originalni, otkazi := context.WithCancel(context.Background())
	odvojeni := context.WithoutCancel(originalni)
	odvojeni, cancel := context.WithTimeout(odvojeni, time.Second)
	defer cancel()

	// otkaži originalni kontekst odmah — kao da je browser konekcija prekinuta
	otkazi()

	k := NoviKlijent(srv.URL, "test-key")
	odgovor, err := k.IzdajRacun(odvojeni, testniZahtev())
	if err != nil {
		t.Fatalf("fiskalni poziv nije smeo da pukne posle otkazivanja originalnog konteksta: %v", err)
	}
	if odgovor.InvoiceNumber != "ПП999" {
		t.Errorf("InvoiceNumber = %q, očekivano ПП999", odgovor.InvoiceNumber)
	}
}

func TestStatusUspeh(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/status" {
			t.Fatalf("neočekivana putanja: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	k := NoviKlijent(srv.URL, "")
	rezultat, err := k.Status(context.Background())
	if err != nil {
		t.Fatalf("Status greška: %v", err)
	}
	if rezultat["status"] != "ok" {
		t.Errorf("status = %v, očekivano ok", rezultat["status"])
	}
}

func TestZakljuciDanNoContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("očekivan DELETE, dobijeno %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	k := NoviKlijent(srv.URL, "")
	if err := k.ZakljuciDan(context.Background()); err != nil {
		t.Fatalf("ZakljuciDan greška: %v", err)
	}
}

func TestNoviKlijentSkidaKrajnjuKosuCrtu(t *testing.T) {
	k := NoviKlijent("http://127.0.0.1:4566/", "kljuc")
	if k.BaseURL != "http://127.0.0.1:4566" {
		t.Errorf("BaseURL = %q, očekivano bez krajnje kose crte", k.BaseURL)
	}
}
