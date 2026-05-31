package main

import (
	"log"
	"net/http"
	"os"

	"ntech/internal/config"
	"ntech/internal/db/sqlite"
	"ntech/internal/handler"

	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()

	if config.JelPrvoPokretanje() {
		config.PokreniSetup()
		return
	}

	port := os.Getenv("NTECH_PORT")
	if port == "" {
		port = "8080"
	}

	putanjaBaze := os.Getenv("NTECH_SQLITE")
	if putanjaBaze == "" {
		putanjaBaze = "ntech.db"
	}

	db, err := sqlite.OtvoriDB(putanjaBaze)
	if err != nil {
		log.Fatalf("Greška pri otvaranju baze: %v", err)
	}
	defer db.Close()

	if err := sqlite.PokreniMigracije(db, "migrations"); err != nil {
		log.Fatalf("Greška pri migracijama: %v", err)
	}
	log.Println("Migracije uspešno izvršene")

	h := handler.Novi(db)

	r := chi.NewRouter()

	// statični fajlovi
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	// rute
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/dashboard", http.StatusFound)
	})
	r.Get("/dashboard", h.Dashboard)
	r.Get("/podesavanja", h.Podesavanja)
	r.Post("/podesavanja/sacuvaj", h.SacuvajPodesavanja)
	r.Get("/tema/{tema}", h.PromeniTemu)

	log.Printf("NTech pokrenut na portu %s", port)
	err = http.ListenAndServe(":"+port, r)
	if err != nil {
		log.Fatalf("Greška: port %s je zauzet ili nije dostupan", port)
	}
}
