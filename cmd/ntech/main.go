package main

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"

	"ntech"
	"ntech/internal/auth"
	"ntech/internal/config"
	"ntech/internal/db/sqlite"
	"ntech/internal/handler"
	ntechmw "ntech/internal/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
)

// Verzija se postavlja pri produkcijskom buildu: go build -ldflags "-X main.Verzija=1.2.0"
var Verzija = "dev"

func main() {
	godotenv.Load("ntech.env")
	auth.InitAuthLog()

	// disk-first logika: ako folder postoji pored binarnog fajla — koristi disk, inače embed.
	// Napomena: templFS i migrFS su rootovani na "." tako da putanje ostaju iste kao u embed-u
	// (npr. "web/templates/stranice/dashboard.html", "migrations/001_kategorije.sql").
	var templFS fs.FS = assets.TemplatesFS
	if _, err := os.Stat("web/templates"); err == nil {
		templFS = os.DirFS(".")
	}
	var migrFS fs.FS = assets.MigracijeFS
	if _, err := os.Stat("migrations"); err == nil {
		migrFS = os.DirFS(".")
	}
	// staticFS je rootovan na "web/static" (isti kao fs.Sub embed-a) — uploads ostaju van embed-a
	var staticFS fs.FS
	if _, err := os.Stat("web/static"); err == nil {
		staticFS = os.DirFS("web/static")
	} else {
		staticFS, _ = fs.Sub(assets.StaticFS, "web/static")
	}

	if config.JelPrvoPokretanje() {
		config.PokreniSetup(templFS)
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

	if err := sqlite.PokreniMigracije(db, migrFS); err != nil {
		log.Fatalf("Greška pri migracijama: %v", err)
	}
	log.Println("Migracije uspešno izvršene")

	// popuni tabelu dozvola podrazumevanim vrednostima ako je prazna
	if err := sqlite.InicijalizujDozvole(context.Background(), db, ntechmw.ImaDozvolu, ntechmw.SveAkcije()); err != nil {
		log.Printf("Upozorenje: greška pri inicijalizaciji dozvola: %v", err)
	}

	napraviStartupBackup(putanjaBaze)

	// periodično brisanje isteklih sesija i starih pokušaja prijave
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		sesijeRepo := sqlite.NoviSesijeRepo(db)
		pokusajiRepo := sqlite.NoviPokusajiPrijaveRepo(db)
		for range ticker.C {
			_ = sesijeRepo.ObrisiIstekle(context.Background())
			_ = pokusajiRepo.ObrisiStare(context.Background(), time.Now().Add(-24*time.Hour))
		}
	}()

	os.MkdirAll("web/static/uploads", 0755)

	h := handler.Novi(db)
	h.Verzija = Verzija
	h.PutanjaBaze = putanjaBaze
	// čuva odabrani FS (disk ili embed) za hot-reload u razvoju i za keš u produkciji
	h.TemplatesFS = templFS

	if os.Getenv("NTECH_ENV") == "production" {
		kes, err := handler.KreirajKes(templFS)
		if err != nil {
			log.Fatalf("Greška pri kreiranju keša šablona: %v", err)
		}
		h.Templates = kes
		log.Printf("Keš šablona kreiran: %d šablona", len(kes))
	}

	r := chi.NewRouter()
	r.Use(ntechmw.BezbednostHeaders())
	r.Use(ntechmw.CsrfMiddleware)
	r.Use(middleware.Compress(5))

	// uploads su uvek na disku — korisnički fajlovi, ne ugrađuju se
	r.Handle("/static/uploads/*", http.StripPrefix("/static/uploads/",
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			http.FileServer(http.Dir("web/static/uploads")).ServeHTTP(w, req)
		})))
	// ostali statični fajlovi: disk ako postoji web/static, inače embed
	r.Handle("/static/*", http.StripPrefix("/static/",
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			http.FileServer(http.FS(staticFS)).ServeHTTP(w, req)
		})))

	// javne rute (bez autentifikacije)
	r.Get("/prijava", h.PrikazPrijave)
	r.Post("/prijava", h.Prijava)
	r.Get("/prijava/totp", h.PrikazTotp)
	r.Post("/prijava/totp", h.VerifikujTotp)
	r.Get("/setup", h.PrikazSetupa)
	r.Post("/setup", h.SacuvajSetup)
	r.Get("/odjava", h.Odjava)

	// zaštićene rute — zahtevaju prijavljenog korisnika
	r.Group(func(r chi.Router) {
		r.Use(ntechmw.RequireAuth(db))

		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/dashboard", http.StatusFound)
		})
		r.Get("/dashboard", h.Dashboard)
		r.Get("/podesavanja", h.Podesavanja)
		r.Get("/admin/podesavanja/opste", h.PodesavanjaOpste)
		r.Get("/admin/podesavanja/izgled", h.PodesavanjaIzgled)
		r.Get("/admin/podesavanja/sistem", h.PodesavanjaSistem)
		r.Post("/podesavanja/sacuvaj", h.SacuvajPodesavanja)
		r.Post("/podesavanja/logo", h.OtpremiLogo)
		r.Post("/podesavanja/login-pozadina", h.OtpremiLoginPozadinu)
		r.Post("/podesavanja/login-pozadina/ukloni", h.UkloniLoginPozadinu)
		r.Post("/podesavanja/login-pozadina/stilovi", h.SacuvajLoginPozadinaStilove)
		r.Post("/podesavanja/app-pozadina", h.OtpremiAppPozadinu)
		r.Post("/podesavanja/app-pozadina/ukloni", h.UkloniAppPozadinu)
		r.Post("/podesavanja/app-pozadina/stilovi", h.SacuvajAppPozadinaStilove)
		r.Get("/podesavanja/backup", h.BackupBaze)
		r.Post("/podesavanja/backup/vrati", h.VratiBackup)
		r.Get("/tema/{tema}", h.PromeniTemu)
		r.Post("/podesavanja/tema", h.PromeniGlobalnuTemu)
		r.Post("/podesavanja/tema/globalno", h.PrimeniGlobalnuTemu)
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
		r.Get("/izvestaji", h.Izvestaji)
		r.Get("/prodaja", h.Prodaja)
		r.Get("/prodaja/nova", h.NovaProdaja)
		r.Post("/prodaja/nova", h.SacuvajProdaju)
		r.Post("/prodaja/obrisi/{id}", h.ObrisiProdaju)
		r.Post("/prodaja/storno/{id}", h.StornoProdaje)
		r.Get("/prodaja/{id}/stampa", h.StampaProdaje)
		r.Get("/prodaja/{id}", h.DetaljiProdaje)

		// podsetnici
		r.Get("/podsetnici", h.Podsetnici)
		r.Get("/podsetnici/novi", h.NoviPodsetnik)
		r.Post("/podsetnici/novi", h.SacuvajPodsetnik)
		r.Get("/podsetnici/izmeni/{id}", h.IzmeniPodsetnik)
		r.Post("/podsetnici/izmeni/{id}", h.SacuvajIzmenePodsetnika)
		r.Post("/podsetnici/zavrseno/{id}", h.OznaciPodsetnik)
		r.Post("/podsetnici/obrisi/{id}", h.ObrisiPodsetnik)

		// rute dostupne adminu i superadminu (superadmin vidi sve, admin ne vidi superadmin naloge)
		r.Group(func(r chi.Router) {
			r.Use(ntechmw.RequireAdmin)
			r.Get("/admin/korisnici", h.AdminKorisnici)
			r.Get("/admin/korisnici/{id}/istorija", h.AdminLoginIstorija)
			r.Post("/admin/korisnici/novi", h.AdminSacuvajKorisnika)
			r.Post("/admin/korisnici/{id}/aktivan", h.AdminToggleAktivan)
			r.Post("/admin/korisnici/{id}/uloga", h.AdminPromeniUlogu)
			r.Post("/admin/korisnici/{id}/obrisi", h.AdminObrisiKorisnika)
			// dozvole — pregled i izmena matrice dostupni adminu, promena uloge samo superadminu
			r.Get("/admin/dozvole", h.AdminDozvole)
			r.Post("/admin/dozvole/sacuvaj", h.AdminDozvoleSacuvaj)
			r.Post("/admin/dozvole/reset", h.AdminDozvoleReset)
		})

		// rute dostupne samo superadminu
		r.Group(func(r chi.Router) {
			r.Use(ntechmw.RequireSuperAdmin)
			r.Post("/admin/dozvole/uloga/{id}", h.AdminDozvolePromeniUlogu)
		})
		r.Get("/admin/profil", h.AdminProfil)
		r.Post("/admin/profil/lozinka", h.AdminPromeniLozinku)
		r.Get("/admin/profil/totp/pokreni", h.AdminTotpPokreni)
		r.Post("/admin/profil/totp/aktiviraj", h.AdminTotpAktivacija)
		r.Post("/admin/profil/totp/deaktiviraj", h.AdminTotpDeaktivacija)
		r.Post("/profil/tema", h.AdminSacuvajLokalnuTemu)
		r.Get("/profil/tema", h.ProfilTema)
		r.Post("/profil/pozadina", h.ProfilOtpremiPozadinu)
		r.Post("/profil/pozadina/ukloni", h.ProfilUkloniPozadinu)
		r.Post("/profil/pozadina/stilovi", h.ProfilSacuvajPozadinuStilove)
	})

	log.Printf("NTech pokrenut na portu %s", port)
	err = http.ListenAndServe(":"+port, r)
	if err != nil {
		log.Fatalf("Greška: port %s je zauzet ili nije dostupan", port)
	}
}

// napraviStartupBackup kreira kopiju baze pri pokretanju i čuva poslednjih 7
func napraviStartupBackup(putanjaBaze string) {
	if _, err := os.Stat(putanjaBaze); os.IsNotExist(err) {
		return
	}

	folder := "backups"
	if err := os.MkdirAll(folder, 0755); err != nil {
		log.Printf("backup: ne mogu kreirati folder: %v", err)
		return
	}

	ime := fmt.Sprintf("ntech_%s.db", time.Now().Format("20060102_150405"))
	odrediste := filepath.Join(folder, ime)

	src, err := os.Open(putanjaBaze)
	if err != nil {
		log.Printf("backup: ne mogu otvoriti bazu: %v", err)
		return
	}
	defer src.Close()

	dst, err := os.Create(odrediste)
	if err != nil {
		log.Printf("backup: ne mogu kreirati backup fajl: %v", err)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		log.Printf("backup: greška pri kopiranju: %v", err)
		return
	}

	log.Printf("Backup kreiran: %s", odrediste)
	ocistiStareBackupe(folder, 7)
}

// ocistiStareBackupe briše najstarije backup fajlove ako ih ima više od max
func ocistiStareBackupe(folder string, max int) {
	fajlovi, err := filepath.Glob(filepath.Join(folder, "ntech_*.db"))
	if err != nil || len(fajlovi) <= max {
		return
	}
	sort.Strings(fajlovi)
	for _, f := range fajlovi[:len(fajlovi)-max] {
		_ = os.Remove(f)
	}
}

