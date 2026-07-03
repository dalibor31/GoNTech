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

// InvoiceRequest je omotač za Teron invoiceRequest JSON objekat. Polja Advance*
// su na vrhu (van invoiceRequest) — Teron ih tako očekuje za konačni račun koji
// zatvara prethodno izdat avansni račun (v. NapraviKonacniZahtevSaAvansom).
type InvoiceRequest struct {
	InvoiceRequest             InvoiceRequestBody `json:"invoiceRequest"`
	AdvancePaid                *float64           `json:"advancePaid,omitempty"`
	AdvanceTax                 *float64           `json:"advanceTax,omitempty"`
	AdvanceLastInvoiceNumber   string             `json:"advanceLastInvoiceNumber,omitempty"`
	AdvanceLastInvoiceDateTime string             `json:"advanceLastInvoiceDateTime,omitempty"`
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

// FinancialSummary je odgovor na GET /api/financial/summary — promet od
// poslednjeg preseka stanja (dnevni pazar), razložen po više kategorija.
type FinancialSummary struct {
	StartOfPeriod         string             `json:"startOfPeriod"`
	EndOfPeriod           string             `json:"endOfPeriod"`
	InvoiceCount          int                `json:"invoiceCount"`
	Total                 float64            `json:"total"`
	TotalCash             float64            `json:"totalCash"`
	TotalByTax            []SummaryTaxStavka `json:"totalByTax"`
	TotalByCashier        []SummaryImeIznos  `json:"totalByCashier"`
	TotalByPaymentType    []SummaryPlacanje  `json:"totalByPaymentType"`
	TotalByArticle        []SummaryArtikal   `json:"totalByArticle"`
	TotalByArticleAdvance []SummaryArtikal   `json:"totalByArticleAdvance"`
}

// SummaryTaxStavka je promet po jednoj poreskoj oznaci/stopi.
type SummaryTaxStavka struct {
	Label    string  `json:"label"`
	Category string  `json:"category"`
	Rate     float64 `json:"rate"`
	Amount   float64 `json:"amount"`
	Osnovica float64 `json:"osnovica"`
}

// SummaryImeIznos je generički par ime-iznos (kasir, i sl.).
type SummaryImeIznos struct {
	Name   string  `json:"name"`
	Amount float64 `json:"amount"`
}

// SummaryPlacanje je promet po vrsti plaćanja (Cash, Card, ...).
type SummaryPlacanje struct {
	PaymentType string  `json:"paymentType"`
	Amount      float64 `json:"amount"`
}

// SummaryArtikal je promet po artiklu.
type SummaryArtikal struct {
	ArticleName string  `json:"articleName"`
	TaxLabel    string  `json:"taxLabel"`
	Amount      float64 `json:"amount"`
	Quantity    float64 `json:"quantity"`
}

// ReportSummaryZahtev je telo zahteva za POST /api/financial/report/summary.
// FromDate/ToDate su opcioni (YYYY-MM-DD) — bez njih izveštaj pokriva period
// od poslednjeg preseka stanja.
type ReportSummaryZahtev struct {
	FromDate string `json:"fromDate,omitempty"`
	ToDate   string `json:"toDate,omitempty"`
	Language string `json:"language,omitempty"`
}

// ReportSummaryOdgovor je odgovor na generisanje dnevnog/periodičnog izveštaja.
type ReportSummaryOdgovor struct {
	ReportPdfBase64 string `json:"reportPdfBase64"`
	ReportName      string `json:"reportName"`
	Filename        string `json:"filename"`
}

// Presek vraća promet od poslednjeg preseka stanja (GET /api/financial/summary).
// Poziv je bezbedan (read-only), ne menja stanje na PFR-u.
func (k *Klijent) Presek(ctx context.Context) (*FinancialSummary, error) {
	url := k.BaseURL + "/api/financial/summary"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("ntech: fiskal.Presek: %w", err)
	}
	k.dodajAuth(req)

	resp, err := k.HTTPKlijent.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ntech: fiskal.Presek: server nedostupan: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		telo, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ntech: fiskal.Presek: HTTP %d: %s", resp.StatusCode, string(telo))
	}

	var rezultat FinancialSummary
	if err := json.NewDecoder(resp.Body).Decode(&rezultat); err != nil {
		return nil, fmt.Errorf("ntech: fiskal.Presek: JSON parse: %w", err)
	}
	return &rezultat, nil
}

// ZakljuciDan svodi promet PFR-a na nulu — dnevni presek stanja
// (DELETE /api/financial/summary). Nepovratna akcija.
func (k *Klijent) ZakljuciDan(ctx context.Context) error {
	url := k.BaseURL + "/api/financial/summary"
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("ntech: fiskal.ZakljuciDan: %w", err)
	}
	k.dodajAuth(req)

	resp, err := k.HTTPKlijent.Do(req)
	if err != nil {
		return fmt.Errorf("ntech: fiskal.ZakljuciDan: server nedostupan: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		telo, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ntech: fiskal.ZakljuciDan: HTTP %d: %s", resp.StatusCode, string(telo))
	}
	return nil
}

// GenerisiIzvestaj traži od PFR-a dnevni/periodični izveštaj u PDF formatu
// (POST /api/financial/report/summary).
func (k *Klijent) GenerisiIzvestaj(ctx context.Context, zahtev ReportSummaryZahtev) (*ReportSummaryOdgovor, error) {
	telo, err := json.Marshal(zahtev)
	if err != nil {
		return nil, fmt.Errorf("ntech: fiskal.GenerisiIzvestaj: marshal: %w", err)
	}

	url := k.BaseURL + "/api/financial/report/summary"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(telo))
	if err != nil {
		return nil, fmt.Errorf("ntech: fiskal.GenerisiIzvestaj: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	k.dodajAuth(req)

	resp, err := k.HTTPKlijent.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ntech: fiskal.GenerisiIzvestaj: server nedostupan: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		teloOdg, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ntech: fiskal.GenerisiIzvestaj: HTTP %d: %s", resp.StatusCode, string(teloOdg))
	}

	var odgovor ReportSummaryOdgovor
	if err := json.NewDecoder(resp.Body).Decode(&odgovor); err != nil {
		return nil, fmt.Errorf("ntech: fiskal.GenerisiIzvestaj: JSON parse: %w", err)
	}
	return &odgovor, nil
}

// dodajAuth postavlja Authorization header ako je API ključ podešen.
func (k *Klijent) dodajAuth(req *http.Request) {
	if k.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+k.APIKey)
	}
}
