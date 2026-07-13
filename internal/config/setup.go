package config

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
)

// nadjiLokalneAdrese vraća sve ne-loopback IPv4 adrese mrežnih kartica
func nadjiLokalneAdrese() []string {
	var adrese []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return adrese
	}
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			if ip4 := ip.To4(); ip4 != nil {
				adrese = append(adrese, ip4.String())
			}
		}
	}
	return adrese
}

// obradiSetupPotvrdi prima izabrani port sa setup stranice, upisuje ntech.env i
// signalizira gotov kanalu da server može da se ugasi. Odbija nevažeći JSON i
// portove van dozvoljenog opsega (v. BUG.md #38 — ranije se greška dekodiranja
// tiho gutala i upisivala port 0).
func obradiSetupPotvrdi(envFajl string, gotov chan<- struct{}) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var telo struct {
			Port int `json:"port"`
		}
		if err := json.NewDecoder(req.Body).Decode(&telo); err != nil || telo.Port <= 0 || telo.Port > 65535 {
			http.Error(w, "Neispravan port", http.StatusBadRequest)
			return
		}
		if err := SacuvajEnv(telo.Port, envFajl); err != nil {
			http.Error(w, "Greška pri čuvanju podešavanja", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		close(gotov)
	}
}

// PokreniSetup pokreće HTTP server za prvo podešavanje i čeka da korisnik završi
func PokreniSetup(fsys fs.FS, envFajl string) {
	port := NadjiSlobodanPort()
	if port == 0 {
		slog.Error("setup: nije pronađen nijedan slobodan port")
		os.Exit(1)
	}

	gotov := make(chan struct{})
	adresa := fmt.Sprintf(":%d", port)
	r := chi.NewRouter()

	// serviraj setup stranicu
	r.Get("/setup", func(w http.ResponseWriter, req *http.Request) {
		portovi := StatusPortova()
		podaci, err := json.Marshal(portovi)
		if err != nil {
			http.Error(w, "Greška", http.StatusInternalServerError)
			return
		}
		sadrzaj, err := fs.ReadFile(fsys, "web/templates/setup/index.html")
		if err != nil {
			http.Error(w, "Greška", http.StatusInternalServerError)
			return
		}
		html := strings.Replace(string(sadrzaj), "PORT_PODACI", string(podaci), 1)
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, html)
	})

	// proverava da li je određeni port slobodan
	r.Get("/setup/proveriPort", func(w http.ResponseWriter, req *http.Request) {
		portStr := req.URL.Query().Get("port")
		var p int
		fmt.Sscanf(portStr, "%d", &p)
		slobodan := JelPortSlobodan(p)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"slobodan": slobodan})
	})

	// prima izabrani port, upisuje ntech.env i signalizira završetak
	r.Post("/setup/potvrdi", obradiSetupPotvrdi(envFajl, gotov))

	srv := &http.Server{Addr: adresa, Handler: r}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("setup server", "error", err)
			os.Exit(1)
		}
	}()

	fmt.Printf("\n╔══════════════════════════════════════════════╗\n")
	fmt.Printf("║  NTech — prvo pokretanje                     ║\n")
	fmt.Printf("║                                              ║\n")
	fmt.Printf("║  Otvorite u browseru:                        ║\n")
	fmt.Printf("║  http://localhost:%d/setup                ║\n", port)
	for _, ip := range nadjiLokalneAdrese() {
		fmt.Printf("║  http://%s:%d/setup                  ║\n", ip, port)
	}
	fmt.Printf("║                                              ║\n")
	fmt.Printf("╚══════════════════════════════════════════════╝\n\n")

	<-gotov
	srv.Shutdown(context.Background())
}
