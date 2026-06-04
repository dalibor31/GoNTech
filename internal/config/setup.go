package config

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/webview/webview_go"
)

// PokreniSetup pokreće WebView prozor za prvo podešavanje
func PokreniSetup(fsys fs.FS) {
	port := NadjiSlobodanPort()
	if port == 0 {
		log.Fatal("Greška: nije pronađen nijedan slobodan port")
	}

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
		var port int
		fmt.Sscanf(portStr, "%d", &port)
		slobodan := JelPortSlobodan(port)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"slobodan": slobodan})
	})

	// prima izabrani port i upisuje .env
	r.Post("/setup/potvrdi", func(w http.ResponseWriter, req *http.Request) {
		var telo struct {
			Port int `json:"port"`
		}
		json.NewDecoder(req.Body).Decode(&telo)
		err := SacuvajEnv(telo.Port)
		if err != nil {
			http.Error(w, "Greška pri čuvanju podešavanja", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})

	// pokreni server u pozadini
	go func() {
		log.Printf("Setup server pokrenut na portu %d", port)
		http.ListenAndServe(adresa, r)
	}()

	// otvori WebView prozor
	w := webview.New(false)
	defer w.Destroy()
	w.SetTitle("NTech — Konfiguracija")
	w.SetSize(520, 580, 0)
	w.Navigate(fmt.Sprintf("http://localhost:%d/setup", port))
	w.Run()
}
