package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"mime"
	"path/filepath"
	"sort"
	"strconv"
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
	mime.AddExtensionType(".js", "text/javascript")
	mime.AddExtensionType(".css", "text/css")
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
	// staticFS je rootovan na "web/static" — u produkciji embed, u razvoju disk
	var staticFS fs.FS
	if os.Getenv("NTECH_ENV") == "production" {
		staticFS, _ = fs.Sub(assets.StaticFS, "web/static")
	} else {
		staticFS = os.DirFS("web/static")
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

	// ukloni zastarele dozvole (siročiće) koje više ne postoje u kodu
	if br, err := sqlite.OcistiSirociceDoz(context.Background(), db, ntechmw.SveAkcije()); err != nil {
		log.Printf("Upozorenje: greška pri čišćenju dozvola: %v", err)
	} else if br > 0 {
		log.Printf("Dozvole: uklonjeno %d zastarelih redova", br)
	}

	// ključ za šifrovanje TOTP tajni u mirovanju (AES-256-GCM)
	totpKljuc, err := ucitajTotpKljuc()
	if err != nil {
		log.Fatalf("Greška pri učitavanju ključa za TOTP: %v", err)
	}

	// jednokratno šifruj eventualne stare TOTP tajne koje su ostale kao čist tekst
	if br, err := sqlite.ZasifrujPostojeceTotp(context.Background(), db, totpKljuc); err != nil {
		log.Printf("Upozorenje: greška pri šifrovanju postojećih TOTP tajni: %v", err)
	} else if br > 0 {
		log.Printf("TOTP: šifrovano %d postojećih tajni", br)
	}

	napraviBackup(db, putanjaBaze)

	// periodični automatski backup — interval se čita iz podešavanja u svakom ciklusu,
	// tako da izmena u podešavanjima stupa na snagu bez restarta (od sledećeg ciklusa)
	go func() {
		for {
			sati := procitajIntPodesavanje(db, "backup_interval_sati", 24)
			time.Sleep(time.Duration(sati) * time.Hour)
			napraviBackup(db, putanjaBaze)
		}
	}()

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

	h := handler.Novi(db, totpKljuc)
	h.Verzija = Verzija
	// verzija statičkih fajlova za cache-busting — menja se pri svakom pokretanju,
	// pa novi build/restart natera brauzer da povuče sveži CSS/JS (umesto starog iz keša)
	h.AssetV = strconv.FormatInt(time.Now().Unix(), 36)
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
	// ostali statični fajlovi: disk ako postoji web/static, inače embed.
	// U produkciji dug immutable keš (URL nosi ?v=verzija za cache-busting pri novom buildu);
	// u razvoju bez keša, da izmene CSS/JS odmah budu vidljive bez ručnog osvežavanja.
	produkcija := os.Getenv("NTECH_ENV") == "production"
	r.Handle("/static/*", http.StripPrefix("/static/",
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if produkcija {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				w.Header().Set("Cache-Control", "no-cache")
			}
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
		r.Use(ntechmw.RequireAuth(db, totpKljuc))

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

		r.Get("/podesavanja/backup", h.BackupBaze)
		r.Post("/podesavanja/backup/vrati", h.VratiBackup)
		r.Get("/magacin", h.Magacin)
		r.Get("/magacin/novi", h.NoviArtikal)
		r.Post("/magacin/novi", h.SacuvajArtikal)
		r.Get("/magacin/izmeni/{id}", h.IzmeniArtikal)
		r.Post("/magacin/izmeni/{id}", h.SacuvajIzmenuArtikla)
		r.Get("/magacin/obrisi/{id}", h.ObrisiArtikal)
		r.Post("/magacin/premesti/{id}", h.PremestiArtikal)
		r.Get("/magacin/kategorije", h.Kategorije)
		r.Post("/magacin/kategorije/dodaj", h.DodajKategoriju)
		r.Get("/magacin/kategorije/obrisi/{id}", h.ObrisiKategoriju)
		r.With(ntechmw.RequireDozvola(h.DozvoleRepo.ImaDozvolu, "nabavka.pregled")).Get("/nabavke", h.Nabavke)
		r.With(ntechmw.RequireDozvola(h.DozvoleRepo.ImaDozvolu, "nabavka.pregled")).Get("/nabavke/nova", h.NovaNabavka)
		r.Post("/nabavke/nova", h.SacuvajNabavku)
		r.With(ntechmw.RequireDozvola(h.DozvoleRepo.ImaDozvolu, "nabavka.pregled")).Get("/nabavke/{id}", h.DetaljiNabavke)
		r.Post("/nabavke/obrisi/{id}", h.ObrisiNabavku)
		r.With(ntechmw.RequireDozvola(h.DozvoleRepo.ImaDozvolu, "dobavljac.pregled")).Get("/dobavljaci", h.Dobavljaci)
		r.Get("/dobavljaci/novi", h.NoviDobavljac)
		r.Post("/dobavljaci/novi", h.SacuvajDobavljaca)
		r.Get("/dobavljaci/izmeni/{id}", h.IzmeniDobavljaca)
		r.Post("/dobavljaci/izmeni/{id}", h.SacuvajIzmeneDobavljaca)
		r.Post("/dobavljaci/obrisi/{id}", h.ObrisiDobavljaca)
		r.With(ntechmw.RequireDozvola(h.DozvoleRepo.ImaDozvolu, "klijent.pregled")).Get("/klijenti", h.Klijenti)
		r.Get("/klijenti/novi", h.NoviKlijent)
		r.Post("/klijenti/novi", h.SacuvajKlijenta)
		r.Get("/klijenti/izmeni/{id}", h.IzmeniKlijenta)
		r.Post("/klijenti/izmeni/{id}", h.SacuvajIzmenuKlijenta)
		r.Post("/klijenti/obrisi/{id}", h.ObrisiKlijenta)
		r.With(ntechmw.RequireDozvola(h.DozvoleRepo.ImaDozvolu, "servis.pregled")).Get("/servis", h.Servis)
		r.Get("/servis/novi", h.NoviNalog)
		r.Post("/servis/novi", h.SacuvajNalog)
		r.Get("/servis/izmeni/{id}", h.IzmeniNalog)
		r.Post("/servis/izmeni/{id}", h.SacuvajIzmenaNaloga)
		r.Post("/servis/obrisi/{id}", h.ObrisiNalog)
		r.With(ntechmw.RequireDozvola(h.DozvoleRepo.ImaDozvolu, "servis.pregled")).Get("/servis/{id}", h.DetaljiNaloga)
		r.Post("/servis/{id}/delovi", h.DodajDeloNalogu)
		r.Post("/servis/{id}/delovi/{deo_id}/obrisi", h.ObrisiDeloNaloga)
		r.Get("/izvestaji", h.Izvestaji)
		r.With(ntechmw.RequireDozvola(h.DozvoleRepo.ImaDozvolu, "prodaja.pregled")).Get("/prodaja", h.Prodaja)
		r.Get("/prodaja/nova", h.NovaProdaja)
		r.Post("/prodaja/nova", h.SacuvajProdaju)
		r.Post("/prodaja/obrisi/{id}", h.ObrisiProdaju)
		r.Post("/prodaja/storno/{id}", h.StornoProdaje)
		r.With(ntechmw.RequireDozvola(h.DozvoleRepo.ImaDozvolu, "prodaja.pregled")).Get("/prodaja/{id}/stampa", h.StampaProdaje)
		r.With(ntechmw.RequireDozvola(h.DozvoleRepo.ImaDozvolu, "prodaja.pregled")).Get("/prodaja/{id}", h.DetaljiProdaje)

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
		r.Post("/profil/tema", h.SacuvajLokalnuTemu)
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

// ucitajTotpKljuc vraća 32-bajtni ključ za šifrovanje TOTP tajni. Čita ga iz
// NTECH_TOTP_KEY (base64). Ako env nije postavljen, generiše nov nasumičan ključ
// i dopisuje ga u ntech.env (perm 0600), tako da postojeće instalacije dobiju
// ključ automatski pri prvom pokretanju nove verzije.
//
// VAŽNO: ako se ovaj ključ izgubi ili promeni, postojeće šifrovane TOTP tajne se
// više ne mogu dešifrovati (korisnici moraju ponovo aktivirati 2FA).
func ucitajTotpKljuc() ([]byte, error) {
	if v := os.Getenv("NTECH_TOTP_KEY"); v != "" {
		kljuc, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return nil, fmt.Errorf("NTECH_TOTP_KEY nije ispravan base64: %w", err)
		}
		if len(kljuc) != auth.DuzinaTotpKljuca {
			return nil, fmt.Errorf("NTECH_TOTP_KEY mora imati %d bajta (trenutno %d)", auth.DuzinaTotpKljuca, len(kljuc))
		}
		return kljuc, nil
	}

	// nema ključa — generiši nov i upiši ga u ntech.env
	kljuc := make([]byte, auth.DuzinaTotpKljuca)
	if _, err := rand.Read(kljuc); err != nil {
		return nil, fmt.Errorf("generisanje ključa: %w", err)
	}
	enkodiran := base64.StdEncoding.EncodeToString(kljuc)
	f, err := os.OpenFile("ntech.env", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("otvaranje ntech.env: %w", err)
	}
	defer f.Close()
	if _, err := f.WriteString("\nNTECH_TOTP_KEY=" + enkodiran + "\n"); err != nil {
		return nil, fmt.Errorf("upis ključa u ntech.env: %w", err)
	}
	os.Setenv("NTECH_TOTP_KEY", enkodiran)
	log.Println("Generisan nov NTECH_TOTP_KEY i upisan u ntech.env")
	return kljuc, nil
}

// napraviBackup kreira konzistentnu kopiju baze i briše najstarije preko zadatog broja kopija.
// Koristi već otvorenu vezu ka bazi (VACUUM INTO je bezbedan na pooled konekciji).
func napraviBackup(db *sql.DB, putanjaBaze string) {
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

	if _, err := db.ExecContext(context.Background(), "VACUUM INTO ?", odrediste); err != nil {
		log.Printf("backup: greška pri pravljenju backup-a: %v", err)
		return
	}

	log.Printf("Backup kreiran: %s", odrediste)
	ocistiStareBackupe(folder, procitajIntPodesavanje(db, "backup_broj_kopija", 7))
}

// procitajIntPodesavanje vraća celobrojnu vrednost podešavanja iz baze,
// ili podrazumevanu ako ključ ne postoji ili nije validan pozitivan broj
func procitajIntPodesavanje(db *sql.DB, kljuc string, podrazumevano int) int {
	v, err := sqlite.DohvatiPodesavanje(context.Background(), db, kljuc)
	if err != nil || v == "" {
		return podrazumevano
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return podrazumevano
	}
	return n
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

