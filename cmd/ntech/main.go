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
	r.Get("/magacin", h.Magacin)
	r.Get("/magacin/novi", h.NoviArtikal)
	r.Post("/magacin/novi", h.SacuvajArtikal)
	r.Get("/magacin/izmeni/{id}", h.IzmeniArtikal)
	r.Post("/magacin/izmeni/{id}", h.SacuvajIzmenuArtikla)
	r.Get("/magacin/obrisi/{id}", h.ObrisiArtikal)
	r.Get("/magacin/kategorije", h.Kategorije)
	r.Post("/magacin/kategorije/dodaj", h.DodajKategoriju)
	r.Get("/magacin/kategorije/obrisi/{id}", h.ObrisiKategoriju)
	r.Get("/nabavke", h.Nabavke)
	r.Get("/nabavke/nova", h.NovaNabavka)
	r.Post("/nabavke/nova", h.SacuvajNabavku)
	r.Get("/nabavke/{id}", h.DetaljiNabavke)
	r.Post("/nabavke/obrisi/{id}", h.ObrisiNabavku)
	r.Get("/dobavljaci", h.Dobavljaci)
	r.Get("/dobavljaci/novi", h.NoviDobavljac)
	r.Post("/dobavljaci/novi", h.SacuvajDobavljaca)
	r.Get("/dobavljaci/izmeni/{id}", h.IzmeniDobavljaca)
	r.Post("/dobavljaci/izmeni/{id}", h.SacuvajIzmeneDobavljaca)
	r.Post("/dobavljaci/obrisi/{id}", h.ObrisiDobavljaca)
	r.Get("/klijenti", h.Klijenti)
	r.Get("/klijenti/novi", h.NoviKlijent)
	r.Post("/klijenti/novi", h.SacuvajKlijenta)
	r.Get("/klijenti/izmeni/{id}", h.IzmeniKlijenta)
	r.Post("/klijenti/izmeni/{id}", h.SacuvajIzmenuKlijenta)
	r.Post("/klijenti/obrisi/{id}", h.ObrisiKlijenta)
	r.Get("/servis", h.Servis)
	r.Get("/servis/novi", h.NoviNalog)
	r.Post("/servis/novi", h.SacuvajNalog)
	r.Get("/servis/izmeni/{id}", h.IzmeniNalog)
	r.Post("/servis/izmeni/{id}", h.SacuvajIzmenaNaloga)
	r.Post("/servis/obrisi/{id}", h.ObrisiNalog)
	r.Get("/servis/{id}", h.DetaljiNaloga)
	r.Get("/prodaja", h.Prodaja)
	r.Get("/prodaja/nova", h.NovaProdaja)
	r.Post("/prodaja/nova", h.SacuvajProdaju)
	r.Post("/prodaja/obrisi/{id}", h.ObrisiProdaju)
	r.Get("/prodaja/{id}/stampa", h.StampaProdaje)
	r.Get("/prodaja/{id}", h.DetaljiProdaje)

	log.Printf("NTech pokrenut na portu %s", port)
	err = http.ListenAndServe(":"+port, r)
	if err != nil {
		log.Fatalf("Greška: port %s je zauzet ili nije dostupan", port)
	}
}
