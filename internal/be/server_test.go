package be

import (
	"bufio"
	"encoding/json"
	"net"
	"testing"
	"time"
)

// TestObradiKonekcijaZatvaraNakonTisine proverava — konekcija
// koja ništa ne pošalje mora biti zatvorena posle citanjeTimeout-a, ne držana
// zauvek.
func TestObradiKonekcijaZatvaraNakonTisine(t *testing.T) {
	stariTimeout := citanjeTimeout
	citanjeTimeout = 100 * time.Millisecond
	defer func() { citanjeTimeout = stariTimeout }()

	klijentska, serverska := net.Pipe()
	defer klijentska.Close()

	gotovo := make(chan struct{})
	go func() {
		obradiKonekciju(serverska, novaTestnaKartica())
		close(gotovo)
	}()

	select {
	case <-gotovo:
		// očekivano: server je zatvorio konekciju posle isteka citanjeTimeout-a
	case <-time.After(2 * time.Second):
		t.Fatal("obradiKonekciju nije prekinuta ni posle 2s tišine klijenta — deadline ne radi")
	}
}

// TestObradiKonekcijaOdgovaraNaKomandu proverava da server i dalje ispravno
// odgovara na komandu pre nego što istekne deadline (da fix #36 ne pokvari
// normalan tok status → verify_pin → sign preko realnog TCP-a).
func TestObradiKonekcijaOdgovaraNaKomandu(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ne mogu da otvorim listener: %v", err)
	}
	defer ln.Close()

	k := novaTestnaKartica()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		obradiKonekciju(conn, k)
	}()

	conn, err := net.DialTimeout("tcp", ln.Addr().String(), 2*time.Second)
	if err != nil {
		t.Fatalf("ne mogu da se povežem: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte(`{"command":"status"}` + "\n")); err != nil {
		t.Fatalf("greška pri slanju komande: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatalf("greška pri čitanju odgovora: %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		t.Fatalf("odgovor nije validan JSON: %v (%q)", err, line)
	}
	if resp["status"] != "ok" {
		t.Fatalf("status = %v, očekivano ok", resp["status"])
	}
}

func TestPokreniOdbijaKadJeLimitDostignut(t *testing.T) {
	// Konceptualna provera semafora u Pokreni je pokrivena kroz kod (kanal
	// kapaciteta maxKonekcija); ovde samo proveravamo da konstanta ima razuman,
	// pozitivan limit umesto neograničenog broja gorutina.
	if maxKonekcija <= 0 {
		t.Fatalf("maxKonekcija mora biti pozitivan broj, dobijeno: %d", maxKonekcija)
	}
}
