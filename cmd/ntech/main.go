package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"ntech/internal/config"
)

func main() {
	if config.JelPrvoPokretanje() {
		config.PokreniSetup()
		return
	}

	// normalno pokretanje
	port := os.Getenv("NTECH_PORT")
	if port == "" {
		port = "8080"
	}

	r := chi.NewRouter()
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Zdravo iz NTech-a")
	})

	log.Printf("NTech pokrenut na portu %s", port)
	err := http.ListenAndServe(":"+port, r)
	if err != nil {
		log.Fatalf("Greška: port %s je zauzet ili nije dostupan", port)
	}
}
