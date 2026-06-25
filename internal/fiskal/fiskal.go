package fiskal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Klijent je HTTP klijent za Teron L-PFR mock server (ili pravi Teron uređaj).
// Komunicira JSON-om preko HTTP POST/GET na baseURL/api/....
type Klijent struct {
	BaseURL     string
	APIKey      string
	HTTPKlijent *http.Client
}

// NoviKlijent kreira novi fiskalni klijent. baseURL je adresa PFR servera
// (npr. "http://127.0.0.1:4566"), apiKey je Bearer token.
func NoviKlijent(baseURL, apiKey string) *Klijent {
	return &Klijent{
		BaseURL:     strings.TrimRight(baseURL, "/"),
		APIKey:      apiKey,
		HTTPKlijent: &http.Client{Timeout: 10 * time.Second},
	}
}

// InvoiceRequest je omotač za Teron invoiceRequest JSON objekat.
type InvoiceRequest struct {
	InvoiceRequest InvoiceRequestBody `json:"invoiceRequest"`
}

// InvoiceRequestBody su podaci koje Teron očekuje unutar invoiceRequest polja.
type InvoiceRequestBody struct {
	InvoiceType            string        `json:"invoiceType"`
	TransactionType        string        `json:"transactionType"`
	Payment                []PaymentItem `json:"payment"`
	Items                  []InvoiceItem `json:"items"`
	Cashier                string        `json:"cashier"`
	BuyerID                string        `json:"buyerId,omitempty"`
	ReferentDocumentNumber string        `json:"referentDocumentNumber,omitempty"`
}

// PaymentItem opisuje jedno plaćanje (može više — keš + kartica).
type PaymentItem struct {
	Amount      float64 `json:"amount"`
	PaymentType string  `json:"paymentType"`
}

// InvoiceItem je jedna stavka fiskalnog računa.
type InvoiceItem struct {
	Name        string   `json:"name"`
	Labels      []string `json:"labels"`
	TotalAmount float64  `json:"totalAmount"`
	UnitPrice   float64  `json:"unitPrice"`
	Quantity    float64  `json:"quantity"`
}

// InvoiceResponse je odgovor PFR servera na zahtev za fiskalizaciju.
// Polja prate stvarni JSON koji Fisk server vraća (vidi NtechFisk.md §3.2).
type InvoiceResponse struct {
	RequestedBy             string    `json:"requestedBy"`
	SignedBy                string    `json:"signedBy"`
	SdcDateTime             string    `json:"sdcDateTime"`
	InvoiceCounter          string    `json:"invoiceCounter"`
	InvoiceCounterExtension string    `json:"invoiceCounterExtension"`
	InvoiceNumber           string    `json:"invoiceNumber"`
	VerificationURL         string    `json:"verificationUrl"`
	VerificationQRCode      string    `json:"verificationQRCode"`
	Journal                 string    `json:"journal"`
	TaxItems                []TaxItem `json:"taxItems"`
	TotalAmount             float64   `json:"totalAmount"`
	TotalTax                float64   `json:"totalTax"`
	Messages                string    `json:"messages"`
}

// TaxItem je stavka poreskog obračuna u fiskalnom odgovoru.
type TaxItem struct {
	Label        string  `json:"label"`
	CategoryName string  `json:"categoryName"`
	CategoryType int     `json:"categoryType"`
	Rate         float64 `json:"rate"`
	Amount       float64 `json:"amount"`
}

// Status vraća trenutni status PFR servera (GET /api/status).
func (k *Klijent) Status(ctx context.Context) (map[string]any, error) {
	url := k.BaseURL + "/api/status"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("ntech: fiskal.Status: %w", err)
	}
	k.dodajAuth(req)

	resp, err := k.HTTPKlijent.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ntech: fiskal.Status: server nedostupan: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		telo, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ntech: fiskal.Status: HTTP %d: %s", resp.StatusCode, string(telo))
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("ntech: fiskal.Status: JSON parse: %w", err)
	}
	return result, nil
}

// IzdajRacun šalje zahtev za fiskalizaciju PFR serveru (POST /api/invoices).
func (k *Klijent) IzdajRacun(ctx context.Context, zahtev InvoiceRequest) (*InvoiceResponse, error) {
	telo, err := json.Marshal(zahtev)
	if err != nil {
		return nil, fmt.Errorf("ntech: fiskal.IzdajRacun: marshal: %w", err)
	}

	url := k.BaseURL + "/api/invoices"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(telo))
	if err != nil {
		return nil, fmt.Errorf("ntech: fiskal.IzdajRacun: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	k.dodajAuth(req)

	resp, err := k.HTTPKlijent.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ntech: fiskal.IzdajRacun: server nedostupan: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		teloOdg, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ntech: fiskal.IzdajRacun: HTTP %d: %s", resp.StatusCode, string(teloOdg))
	}

	var odgovor InvoiceResponse
	if err := json.NewDecoder(resp.Body).Decode(&odgovor); err != nil {
		return nil, fmt.Errorf("ntech: fiskal.IzdajRacun: JSON parse: %w", err)
	}
	return &odgovor, nil
}

// dodajAuth postavlja Authorization header ako je API ključ podešen.
func (k *Klijent) dodajAuth(req *http.Request) {
	if k.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+k.APIKey)
	}
}
