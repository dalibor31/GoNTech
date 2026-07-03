// Package be implementira emulator bezbednosnog elementa (pametne kartice).
// Sluša na TCP portu i obrađuje 4 JSON komande: status, certificate, verify_pin, sign.
// Podatke o firmi čita iz NTech SQLite baze pri pokretanju.
package be

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"ntech/internal/db/sqlite"
)

// tipExt mapira (invoiceType, transactionType) → (ćirilični sufiks, ključ brojača)
var tipExt = map[[2]string][2]string{
	{"Normal", "Sale"}:     {"ПП", "pp"},
	{"Normal", "Refund"}:   {"ПР", "pr"},
	{"Advance", "Sale"}:    {"АП", "ap"},
	{"Advance", "Refund"}:  {"АР", "ar"},
	{"Copy", "Sale"}:       {"КП", "kp"},
	{"Copy", "Refund"}:     {"КР", "kr"},
	{"Training", "Sale"}:   {"ОП", "op"},
	{"Training", "Refund"}: {"ОР", "or"},
	{"Proforma", "Sale"}:   {"РП", "rp"},
	{"Proforma", "Refund"}: {"РР", "rr"},
}

// Kartica drži stanje emuliranog bezbednosnog elementa.
type Kartica struct {
	mu sync.Mutex
	db *sql.DB

	// identitet (zamrznuto pri "personalizaciji" — čitamo iz DB pri pokretanju)
	JID            string
	PIN            string
	TIN            string // RS+PIB
	TINPlain       string // samo PIB
	Name           string
	Address        string
	City           string
	District       string
	BusinessUnitID string
	LocationName   string
	ValidFrom      string
	ValidTo        string
	Issuer         string

	// stanje u toku rada
	pinUnesen    bool
	totalCounter int
	counters     map[string]int
	Limit        float64
	unreadAmount float64
}

func novaKartica(db *sql.DB) *Kartica {
	k := &Kartica{
		db:     db,
		JID:    env("BE_JID", "TRNMOCK1"),
		PIN:    env("BE_PIN", "1234"),
		Limit:  envFloat("BE_LIMIT", 500000),
		Issuer: "Poreska uprava RS",
		counters: map[string]int{
			"pp": 0, "pr": 0, "ap": 0, "ar": 0,
			"kp": 0, "kr": 0, "op": 0, "or": 0,
			"rp": 0, "rr": 0,
		},
	}

	now := time.Now()
	k.ValidFrom = now.Format("2006-01-02T00:00:00+01:00")
	k.ValidTo = now.AddDate(3, 0, 0).Format("2006-01-02T00:00:00+01:00")

	// čitaj podatke firme, PIN i limit iz baze (env var ima prednost)
	k.ucitajFirmu(db)

	slog.Info("kartica emulator inicijalizovan",
		"jid", k.JID,
		"tin", k.TINPlain,
		"firma", k.Name,
		"limit", k.Limit,
	)
	return k
}

func (k *Kartica) ucitajFirmu(db *sql.DB) {
	rows, err := db.Query(
		"SELECT kljuc, vrednost FROM podesavanja WHERE kljuc IN " +
			"('naziv_firme','pib','maticni_broj','adresa','telefon'," +
			"'poslovna_jedinica_naziv','poslovna_jedinica_oznaka','opstina','grad'," +
			"'be_pin','be_limit','be_total_counter','be_counters_json','be_unread_amount')",
	)
	if err != nil {
		slog.Warn("be: ne mogu da čitam podesavanja", "error", err)
		k.postaviTestPodatke()
		return
	}
	defer rows.Close()

	p := map[string]string{}
	for rows.Next() {
		var k2, v string
		if err := rows.Scan(&k2, &v); err == nil {
			p[k2] = v
		}
	}

	naziv := p["naziv_firme"]
	if naziv == "" {
		naziv = "Test Company DOO"
	}
	pib := p["pib"]
	if pib == "" {
		pib = "123456789"
	}

	k.Name = naziv
	k.TINPlain = pib
	k.TIN = "RS" + pib
	k.Address = p["adresa"]
	if k.Address == "" {
		k.Address = "Test Adresa 1"
	}
	k.City = p["grad"]
	if k.City == "" {
		k.City = "Beograd"
	}
	k.District = p["opstina"]
	if k.District == "" {
		k.District = "Savski Venac"
	}
	k.LocationName = p["poslovna_jedinica_naziv"]
	if k.LocationName == "" {
		k.LocationName = naziv
	}
	k.BusinessUnitID = p["poslovna_jedinica_oznaka"]
	if k.BusinessUnitID == "" {
		k.BusinessUnitID = "BU-001"
	}

	// PIN i limit iz baze — samo ako env var nije postavljen
	if os.Getenv("BE_PIN") == "" {
		if pin := p["be_pin"]; pin != "" {
			k.PIN = pin
		}
	}
	if os.Getenv("BE_LIMIT") == "" {
		if lim := p["be_limit"]; lim != "" {
			if f, err := strconv.ParseFloat(lim, 64); err == nil && f > 0 {
				k.Limit = f
			}
		}
	}

	// brojači se pamte preko restarta — fiskalni brojač NIKAD ne sme da krene
	// ispočetka, to bi proizvelo duplirane PFR brojeve za stvarne (ili mock) račune
	if tc := p["be_total_counter"]; tc != "" {
		if n, err := strconv.Atoi(tc); err == nil {
			k.totalCounter = n
		}
	}
	if cj := p["be_counters_json"]; cj != "" {
		var sacuvani map[string]int
		if err := json.Unmarshal([]byte(cj), &sacuvani); err == nil {
			for key, val := range sacuvani {
				k.counters[key] = val
			}
		}
	}
	if ua := p["be_unread_amount"]; ua != "" {
		if f, err := strconv.ParseFloat(ua, 64); err == nil {
			k.unreadAmount = f
		}
	}
}

// sacuvajBrojace upisuje trenutno stanje brojača u podesavanja, da posle restarta
// emulatora fiskalni brojač nastavi odakle je stao (poziva se pod k.mu iz cmdSign/
// cmdResetAudit).
func (k *Kartica) sacuvajBrojace() {
	if k.db == nil {
		return
	}
	ctx := context.Background()
	if err := sqlite.SacuvajPodesavanje(ctx, k.db, "be_total_counter", strconv.Itoa(k.totalCounter)); err != nil {
		slog.Warn("be: ne mogu da sačuvam brojač", "error", err)
		return
	}
	if cj, err := json.Marshal(k.counters); err == nil {
		_ = sqlite.SacuvajPodesavanje(ctx, k.db, "be_counters_json", string(cj))
	}
	_ = sqlite.SacuvajPodesavanje(ctx, k.db, "be_unread_amount", strconv.FormatFloat(k.unreadAmount, 'f', -1, 64))
}

func (k *Kartica) postaviTestPodatke() {
	k.Name = "Test Company DOO"
	k.TINPlain = "123456789"
	k.TIN = "RS123456789"
	k.Address = "Test Adresa 1"
	k.City = "Beograd"
	k.District = "Savski Venac"
	k.LocationName = "Test Company DOO"
	k.BusinessUnitID = "BU-001"
}

// ── Komande ────────────────────────────────────────────────

func (k *Kartica) cmdStatus() map[string]any {
	k.mu.Lock()
	defer k.mu.Unlock()
	counters := map[string]int{}
	for key, val := range k.counters {
		counters[key] = val
	}
	return map[string]any{
		"status":        "ok",
		"total_counter": k.totalCounter,
		"counters":      counters,
		"limit":         k.Limit,
		"unread_amount": k.unreadAmount,
		"pin_required":  !k.pinUnesen,
	}
}

func (k *Kartica) cmdCertificate() map[string]any {
	k.mu.Lock()
	defer k.mu.Unlock()
	return map[string]any{
		"status":           "ok",
		"jid":              k.JID,
		"tin":              k.TIN,
		"tin_plain":        k.TINPlain,
		"name":             k.Name,
		"address":          k.Address,
		"city":             k.City,
		"district":         k.District,
		"business_unit_id": k.BusinessUnitID,
		"location_name":    k.LocationName,
		"valid_from":       k.ValidFrom,
		"valid_to":         k.ValidTo,
		"issuer":           k.Issuer,
	}
}

func (k *Kartica) cmdVerifyPin(pin string) map[string]any {
	k.mu.Lock()
	defer k.mu.Unlock()
	if subtle.ConstantTimeCompare([]byte(pin), []byte(k.PIN)) != 1 {
		return map[string]any{"status": "error", "code": "2100", "message": "Pogrešan PIN"}
	}
	k.pinUnesen = true
	return map[string]any{"status": "ok"}
}

func (k *Kartica) cmdResetAudit() map[string]any {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.unreadAmount = 0
	k.sacuvajBrojace()
	return map[string]any{"status": "ok", "unread_amount": 0}
}

func (k *Kartica) cmdSign(invoiceType, transactionType string, totalAmount float64) map[string]any {
	k.mu.Lock()
	defer k.mu.Unlock()

	// blokada limite
	if k.unreadAmount >= k.Limit {
		return map[string]any{
			"status":  "blocked",
			"message": fmt.Sprintf("Limit iščitavanja dostignut (%.2f / %.2f)", k.unreadAmount, k.Limit),
		}
	}

	// određi tip i ext
	ext, tipKey := "", "pp"
	if v, ok := tipExt[[2]string{invoiceType, transactionType}]; ok {
		ext = v[0]
		tipKey = v[1]
	} else {
		ext = "ПП"
		tipKey = "pp"
	}

	k.totalCounter++
	k.counters[tipKey]++
	typCounter := k.counters[tipKey]

	// za fiskalne račune akumuliraj neisčitani iznos
	nonFiscal := strings.HasPrefix(tipKey, "k") || strings.HasPrefix(tipKey, "o") || strings.HasPrefix(tipKey, "r")
	if !nonFiscal && transactionType == "Sale" {
		k.unreadAmount += totalAmount
	} else if !nonFiscal && transactionType == "Refund" {
		k.unreadAmount -= totalAmount
		if k.unreadAmount < 0 {
			k.unreadAmount = 0
		}
	}

	k.sacuvajBrojace()

	// lažni potpis — random base64 dovoljno dug da izgleda realistično
	sig := mockPotpis()

	return map[string]any{
		"status":            "ok",
		"counter":           k.totalCounter,
		"counter_extension": ext,
		"type_counter":      typCounter,
		"signature":         sig,
		"blocked":           false,
	}
}

// mockPotpis generiše 64-bajtni nasumični base64 string koji imitira RSA potpis.
func mockPotpis() string {
	b := make([]byte, 64)
	for i := range b {
		b[i] = byte(rand.IntN(256))
	}
	return base64.StdEncoding.EncodeToString(b)
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}
