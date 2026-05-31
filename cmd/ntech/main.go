package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"ntech/internal/config"
	"ntech/internal/db/sqlite"

	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
)

func main() {
	// učitaj .env fajl ako postoji
	godotenv.Load()

	if config.JelPrvoPokretanje() {
		config.PokreniSetup()
		return
	}

	// normalno pokretanje
	port := os.Getenv("NTECH_PORT")
	if port == "" {
		port = "8080"
	}

	// putanja do SQLite fajla
	putanjaBaze := os.Getenv("NTECH_SQLITE")
	if putanjaBaze == "" {
		putanjaBaze = "ntech.db"
	}

	// otvaramo konekciju ka bazi
	db, err := sqlite.OtvoriDB(putanjaBaze)
	if err != nil {
		log.Fatalf("Greška pri otvaranju baze: %v", err)
	}
	defer db.Close()

	// pokrećemo migracije
	if err := sqlite.PokreniMigracije(db, "migrations"); err != nil {
		log.Fatalf("Greška pri migracijama: %v", err)
	}
	log.Println("Migracije uspešno izvršene")

	r := chi.NewRouter()
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Zdravo iz NTech-a")
	})

	log.Printf("NTech pokrenut na portu %s", port)
	err = http.ListenAndServe(":"+port, r)
	if err != nil {
		log.Fatalf("Greška: port %s je zauzet ili nije dostupan", port)
	}
}
