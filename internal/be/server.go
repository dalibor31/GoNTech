package be

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"time"
)

// maxKonekcija ograničava broj istovremenih TCP konekcija — bez ovoga bi klijent
// koji samo otvori konekciju i ništa ne pošalje mogao da drži neograničen broj
// gorutina/file descriptor-a.
const maxKonekcija = 32

// citanjeTimeout je rok po redu (komandi) koji se osvežava pre svakog čitanja —
// klijent koji ćuti duže od ovoga se prekida umesto da drži konekciju zauvek.
// var (ne const) da bi testovi mogli da ga privremeno skrate.
var citanjeTimeout = 30 * time.Second

// JeUkljucen vraća true ako emulator treba da se pokrene.
// Prioritet: env var BE_ENABLED > DB podesavanje be_enabled > podrazumevano true.
func JeUkljucen(db *sql.DB) bool {
	if v := os.Getenv("BE_ENABLED"); v != "" {
		return v != "false" && v != "0"
	}
	var vrednost string
	_ = db.QueryRow("SELECT vrednost FROM podesavanja WHERE kljuc='be_enabled'").Scan(&vrednost)
	return vrednost != "false" && vrednost != "0"
}

// Pokreni startuje TCP listener za kartica emulator.
// Blokira dok listener ne padne — pozivati kao goroutine.
func Pokreni(db *sql.DB) {
	port := env("BE_PORT", "4567")
	addr := fmt.Sprintf("0.0.0.0:%s", port)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("be: ne mogu da pokrenem listener", "addr", addr, "error", err)
		os.Exit(1)
	}
	slog.Info("kartica emulator sluša", "addr", addr)

	k := novaKartica(db)
	sem := make(chan struct{}, maxKonekcija)

	for {
		conn, err := ln.Accept()
		if err != nil {
			slog.Warn("be: greška pri Accept()", "error", err)
			continue
		}
		select {
		case sem <- struct{}{}:
			go func() {
				defer func() { <-sem }()
				obradiKonekciju(conn, k)
			}()
		default:
			slog.Warn("be: dostignut limit konekcija, odbijam klijenta", "limit", maxKonekcija)
			conn.Close()
		}
	}
}

func obradiKonekciju(conn net.Conn, k *Kartica) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	for {
		// SetDeadline (ne samo SetReadDeadline) da rok pokrije i odgovori() Write
		// koji sledi posle čitanja — klijent koji šalje komande a nikad ne čita
		// odgovore bi inače mogao da blokira Write zauvek i iscrpi semafor konekcija.
		conn.SetDeadline(time.Now().Add(citanjeTimeout))
		if !scanner.Scan() {
			return
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req map[string]any
		if err := json.Unmarshal(line, &req); err != nil {
			odgovori(conn, map[string]any{"status": "error", "message": "invalid JSON"})
			return
		}

		cmd, _ := req["command"].(string)
		var resp map[string]any

		switch cmd {
		case "status":
			resp = k.cmdStatus()

		case "certificate":
			resp = k.cmdCertificate()

		case "verify_pin":
			pin, _ := req["pin"].(string)
			resp = k.cmdVerifyPin(pin)

		case "reset_audit":
			resp = k.cmdResetAudit()

		case "sign":
			invType, _ := req["invoice_type"].(string)
			txType, _ := req["transaction_type"].(string)
			totalAmount, _ := req["total_amount"].(float64)
			resp = k.cmdSign(invType, txType, totalAmount)

		default:
			resp = map[string]any{"status": "error", "message": fmt.Sprintf("nepoznata komanda: %q", cmd)}
		}

		odgovori(conn, resp)
	}
}

func odgovori(conn net.Conn, resp map[string]any) {
	b, err := json.Marshal(resp)
	if err != nil {
		return
	}
	b = append(b, '\n')
	conn.Write(b)
}
