package handler

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
)

// FiskalVlPodaci su podaci dekodirani iz vl parametra QR koda.
type FiskalVlPodaci struct {
	InvoiceNumber   string          `json:"n"`
	InvoiceCounter  string          `json:"ic"`
	DateTime        string          `json:"t"`
	TotalAmount     float64         `json:"a"`
	TIN             string          `json:"c"`
	Company         string          `json:"co"`
	Store           string          `json:"lo"`
	Address         string          `json:"ad"`
	City            string          `json:"g"`
	District        string          `json:"di"`
	InvoiceType     string          `json:"it"`
	TransactionType string          `json:"tr"`
	TaxItems        []FiskalTaxItem `json:"tx"`
	Payments        []FiskalPayment `json:"pm"`
	Cashier         string          `json:"ca"`
	BuyerID         string          `json:"bi"`
	Items           []FiskalItem    `json:"items"`
}

type FiskalTaxItem struct {
	Label        string  `json:"label"`
	CategoryName string  `json:"categoryName"`
	Rate         float64 `json:"rate"`
	Base         float64 `json:"base"`
	Amount       float64 `json:"amount"`
}

type FiskalPayment struct {
	Type   string  `json:"type"`
	Amount float64 `json:"amount"`
}

type FiskalItem struct {
	Name        string   `json:"name"`
	Quantity    float64  `json:"quantity"`
	UnitPrice   float64  `json:"unitPrice"`
	TotalAmount float64  `json:"totalAmount"`
	Labels      []string `json:"labels"`
}

type fiskalVerifikacijaPodaci struct {
	Podaci     *FiskalVlPodaci
	Greska     string
	JeFiskalni bool
}

func (h *Handler) FiskalVerifikacija(w http.ResponseWriter, r *http.Request) {
	vl := r.URL.Query().Get("vl")

	podaci := &fiskalVerifikacijaPodaci{}

	if vl == "" {
		podaci.Greska = "Nedostaje vl parametar."
		h.renderujStandalone(w, "fiskal_verifikacija", podaci)
		return
	}

	decoded, err := base64.StdEncoding.DecodeString(vl)
	if err != nil {
		// proba RawStdEncoding (bez paddinga) i URLEncoding
		decoded, err = base64.RawStdEncoding.DecodeString(vl)
		if err != nil {
			podaci.Greska = "Neispravan QR kod."
			h.renderujStandalone(w, "fiskal_verifikacija", podaci)
			return
		}
	}

	var vlPodaci FiskalVlPodaci
	if err := json.Unmarshal(decoded, &vlPodaci); err != nil {
		podaci.Greska = "Neispravan format podataka."
		h.renderujStandalone(w, "fiskal_verifikacija", podaci)
		return
	}

	podaci.Podaci = &vlPodaci
	podaci.JeFiskalni = vlPodaci.InvoiceType != "Copy" &&
		vlPodaci.InvoiceType != "Training" &&
		vlPodaci.InvoiceType != "Proforma"

	h.renderujStandalone(w, "fiskal_verifikacija", podaci)
}
