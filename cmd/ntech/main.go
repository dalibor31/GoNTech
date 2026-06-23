package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"ntech"
	"ntech/internal/auth"
	"ntech/internal/config"
	"ntech/internal/db"
	"ntech/internal/db/sqlite"
	"ntech/internal/handler"
	ntechmw "ntech/internal/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
)

// Verzija se postavlja pri produkcijskom buildu: go build -ldflags "-X main.Verzija=1.2.0"
var Verzija = "dev"

// podesiLog postavlja podrazumevani slog logger: JSON u produkciji (za mašinsku
// obradu logova), tekst u razvoju (čitljivije). Nivo je Info.
func podesiLog() {
	opcije := &slog.HandlerOptions{Level: slog.LevelInfo}
	var handler slog.Handler
	if os.Getenv("NTECH_ENV") == "production" {
		handler = slog.NewJSONHandler(os.Stdout, opcije)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opcije)
	}
	slog.SetDefault(slog.New(handler))
}

func main() {
	mime.AddExtensionType(".js", "text/javascript")
	mime.AddExtensionType(".css", "text/css")
	putanjaBaze := os.Getenv("NTECH_SQLITE")
	if putanjaBaze == "" {
		putanjaBaze = "ntech.db"
	}
	envFajl := config.PutanjaNtechEnv(putanjaBaze)
	// u production/demo modu sve podešavanja dolaze iz env promenljivih —
	// kreiraj prazan fajl ako ne postoji da se ne pokrene setup wizard
	if env := os.Getenv("NTECH_ENV"); env == "production" || env == "demo" {
		if _, err := os.Stat(envFajl); os.IsNotExist(err) {
			os.WriteFile(envFajl, []byte(""), 0600)
		}
	}
	godotenv.Load(envFajl)
	podesiLog()
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
	// staticFS je rootovan na "web/static" — u produkciji i demo modu embed, u razvoju disk
	var staticFS fs.FS
	if ntechEnv := os.Getenv("NTECH_ENV"); ntechEnv == "production" || ntechEnv == "demo" {
		staticFS, _ = fs.Sub(assets.StaticFS, "web/static")
	} else {
		staticFS = os.DirFS("web/static")
	}

	if config.JelPrvoPokretanje(envFajl) {
		config.PokreniSetup(templFS, envFajl)
		return
	}

	port := os.Getenv("NTECH_PORT")
	if port == "" {
		port = "8080"
	}

	db, err := sqlite.OtvoriDB(putanjaBaze)
	if err != nil {
		slog.Error("Greška pri otvaranju baze", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := sqlite.PokreniMigracije(db, migrFS); err != nil {
		slog.Error("Greška pri migracijama", "error", err)
		os.Exit(1)
	}
	slog.Info("migracije uspešno izvršene")

	// popuni tabelu dozvola podrazumevanim vrednostima ako je prazna
	if err := sqlite.InicijalizujDozvole(context.Background(), db, ntechmw.ImaDozvolu, ntechmw.SveAkcije()); err != nil {
		slog.Warn("greška pri inicijalizaciji dozvola", "error", err)
	}

	// ukloni zastarele dozvole (siročiće) koje više ne postoje u kodu
	if br, err := sqlite.OcistiSirociceDoz(context.Background(), db, ntechmw.SveAkcije()); err != nil {
		slog.Warn("greška pri čišćenju dozvola", "error", err)
	} else if br > 0 {
		slog.Info("uklonjene zastarele dozvole", "broj", br)
	}

	// ključ za šifrovanje TOTP tajni u mirovanju (AES-256-GCM)
	totpKljuc, err := ucitajTotpKljuc(envFajl)
	if err != nil {
		slog.Error("Greška pri učitavanju ključa za TOTP", "error", err)
		os.Exit(1)
	}

	// jednokratno šifruj eventualne stare TOTP tajne koje su ostale kao čist tekst
	if br, err := sqlite.ZasifrujPostojeceTotp(context.Background(), db, totpKljuc); err != nil {
		slog.Warn("greška pri šifrovanju postojećih TOTP tajni", "error", err)
	} else if br > 0 {
		slog.Info("šifrovane postojeće TOTP tajne", "broj", br)
	}

	{
		max := procitajIntPodesavanje(db, "backup_broj_kopija", 7)
		if os.Getenv("NTECH_ENV") == "demo" {
			max = 2
		}
		napraviBackup(db, putanjaBaze, max)
	}

	os.MkdirAll("web/static/uploads", 0755)

	h := handler.Novi(db, totpKljuc)
	h.Verzija = Verzija
	h.JelDemo = os.Getenv("NTECH_ENV") == "demo"
	if h.JelDemo {
		h.Verzija = "DEMO verzija"
		if err := postaviDemoKorisnika(context.Background(), h.KorisniciRepo); err != nil {
			slog.Warn("demo: greška pri postavljanju demo korisnika", "error", err)
		}
	}
	// verzija statičkih fajlova za cache-busting — menja se pri svakom pokretanju,
	// pa novi build/restart natera brauzer da povuče sveži CSS/JS (umesto starog iz keša)
	h.AssetV = strconv.FormatInt(time.Now().Unix(), 36)
	h.PutanjaBaze = putanjaBaze
	// čuva odabrani FS (disk ili embed) za hot-reload u razvoju i za keš u produkciji
	h.TemplatesFS = templFS

	if ntechEnv := os.Getenv("NTECH_ENV"); ntechEnv == "production" || ntechEnv == "demo" {
		kes, err := handler.KreirajKes(templFS)
		if err != nil {
			slog.Error("Greška pri kreiranju keša šablona", "error", err)
			os.Exit(1)
		}
		h.Templates = kes
		slog.Info("keš šablona kreiran", "broj", len(kes))
	}

	// Pozadinske gorutine se pokreću posle kreiranja h i rade preko h.SaBazom,
	// pa uvek koriste TRENUTNU konekciju baze (posle obnove backupa h.DB se menja).

	// periodični automatski backup — interval se čita iz podešavanja u svakom
	// ciklusu, pa izmena stupa na snagu bez restarta (od sledećeg ciklusa)
	go func() {
		for {
			sati := 24
			h.SaBazom(func(db *sql.DB) {
				sati = procitajIntPodesavanje(db, "backup_interval_sati", 24)
			})
			time.Sleep(time.Duration(sati) * time.Hour)
			h.SaBazom(func(db *sql.DB) {
				max := procitajIntPodesavanje(db, "backup_broj_kopija", 7)
				if h.JelDemo {
					max = 2
				}
				napraviBackup(db, putanjaBaze, max)
			})
		}
	}()

	// periodično brisanje isteklih sesija i starih pokušaja prijave
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			h.SaBazom(func(db *sql.DB) {
				_ = sqlite.NoviSesijeRepo(db).ObrisiIstekle(context.Background())
				_ = sqlite.NoviPokusajiPrijaveRepo(db).ObrisiStare(context.Background(), time.Now().Add(-24*time.Hour))
			})
		}
	}()

	r := chi.NewRouter()
	r.Use(ntechmw.BezbednostHeaders())
	r.Use(ntechmw.CsrfMiddleware)
	r.Use(middleware.Compress(5))
	// deljeno zaključavanje baze za vreme zahteva — obnova backupa (VratiBackup)
	// čeka da svi zahtevi završe pre zamene konekcije (vidi handler.ZakljucajCitanje)
	r.Use(h.ZakljucajCitanje)

	// uploads su uvek na disku — korisnički fajlovi, ne ugrađuju se
	r.Handle("/static/uploads/*", http.StripPrefix("/static/uploads/",
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			http.FileServer(http.Dir("web/static/uploads")).ServeHTTP(w, req)
		})))
	// ostali statični fajlovi: disk ako postoji web/static, inače embed.
	// U produkciji dug immutable keš (URL nosi ?v=verzija za cache-busting pri novom buildu);
	// u razvoju bez keša, da izmene CSS/JS odmah budu vidljive bez ručnog osvežavanja.
	ntechEnvStr := os.Getenv("NTECH_ENV")
	produkcija := ntechEnvStr == "production" || ntechEnvStr == "demo"
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
	r.Get("/status/{token}", h.ServisJavniStatus)

	// zaštićene rute — zahtevaju prijavljenog korisnika
	r.Group(func(r chi.Router) {
		r.Use(ntechmw.RequireAuth(db, totpKljuc))

		// doz vraća middleware koji na ruteru zahteva datu dozvolu za mutirajuću
		// rutu (403 ako uloga nema dozvolu). Ruter je time garantovani sloj zaštite
		// — zaboravljena provera u handleru ne ostavlja endpoint nezaštićenim.
		doz := func(akcija string) func(http.Handler) http.Handler {
			return ntechmw.RequireDozvolaMut(h.DozvoleRepo.ImaDozvolu, akcija)
		}

		// modul vraća middleware koji propušta zahtev samo ako je zakonski modul
		// uključen za firmu (profil firme). Sloj IZNAD RBAC-a — zahtev mora proći
		// i „modul uključen" (ovo) i „korisnik sme" (doz/zahtevajDozvolu).
		proveriModul := func(ctx context.Context, m string) bool {
			pod, err := sqlite.DohvatiSvaPodesavanja(ctx, h.DB)
			if err != nil {
				return false
			}
			return config.ModulUkljucen(pod, m)
		}
		modul := func(m string) func(http.Handler) http.Handler {
			return ntechmw.RequireModul(proveriModul, m)
		}

		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/dashboard", http.StatusFound)
		})
		r.Get("/dashboard", h.Dashboard)
		r.Get("/podesavanja", h.Podesavanja)
		r.Get("/admin/podesavanja/opste", h.PodesavanjaOpste)
		r.Get("/admin/podesavanja/izgled", h.PodesavanjaIzgled)
		r.Get("/admin/podesavanja/sistem", h.PodesavanjaSistem)
		r.Get("/admin/podesavanja/servis", h.PodesavanjaServis)
		r.Get("/admin/podesavanja/kalkulacija-pdv", h.PdvStope)
		r.With(doz("podesavanja.izmeni")).Post("/podesavanja/pdv-stope/dodaj", h.DodajPdvStopu)
		r.With(doz("podesavanja.izmeni")).Post("/podesavanja/pdv-stope/{id}/izmeni", h.IzmeniPdvStopu)
		r.With(doz("podesavanja.izmeni")).Post("/podesavanja/pdv-stope/{id}/aktivnost", h.PromeniAktivnostPdvStope)
		r.With(doz("podesavanja.izmeni")).Post("/podesavanja/sacuvaj", h.SacuvajPodesavanja)
		r.With(doz("podesavanja.izmeni")).Post("/podesavanja/logo", h.OtpremiLogo)
		r.With(doz("podesavanja.izmeni")).Post("/podesavanja/logo/ukloni", h.UkloniLogo)
		r.With(doz("podesavanja.login_pozadina")).Post("/podesavanja/login-pozadina", h.OtpremiLoginPozadinu)
		r.With(doz("podesavanja.login_pozadina")).Post("/podesavanja/login-pozadina/ukloni", h.UkloniLoginPozadinu)
		r.With(doz("podesavanja.login_pozadina")).Post("/podesavanja/login-pozadina/stilovi", h.SacuvajLoginPozadinaStilove)

		r.Get("/podesavanja/backup", h.BackupBaze)
		r.With(doz("backup.pokreni")).Post("/podesavanja/backup/vrati", h.VratiBackup)

		// PDV evidencija — KIR (knjiga izdatih računa). Dostupno samo kada je modul
		// „pdv" uključen za firmu (RequireModul), uz RBAC dozvolu pdv.*.
		r.With(modul("pdv")).Get("/pdv/kir", h.PdvKir)
		r.With(modul("pdv")).Get("/pdv/kir/nova", h.NoviPdvKir)
		r.With(modul("pdv"), doz("pdv.dodaj")).Post("/pdv/kir/nova", h.SacuvajPdvKir)
		r.With(modul("pdv"), doz("pdv.obrisi")).Post("/pdv/kir/obrisi/{id}", h.ObrisiPdvKir)
		r.With(modul("pdv")).Get("/pdv/kpr", h.PdvKpr)
		r.With(modul("pdv")).Get("/pdv/kpr/nova", h.NoviPdvKpr)
		r.With(modul("pdv"), doz("pdv.dodaj")).Post("/pdv/kpr/nova", h.SacuvajPdvKpr)
		r.With(modul("pdv"), doz("pdv.obrisi")).Post("/pdv/kpr/obrisi/{id}", h.ObrisiPdvKpr)
		r.With(modul("pdv")).Get("/pdv/obracun", h.PdvObracunStranica)
		r.Get("/magacin", h.Magacin)
		r.Get("/usluge", h.Usluge)
		r.Get("/usluge/nova", h.NovaUsluga)
		r.With(doz("artikal.dodaj")).Post("/usluge/nova", h.SacuvajUslugu)
		r.Get("/usluge/izmeni/{id}", h.IzmeniUslugu)
		r.With(doz("artikal.izmeni")).Post("/usluge/izmeni/{id}", h.SacuvajIzmenuUsluge)
		r.With(doz("artikal.obrisi")).Post("/usluge/obrisi/{id}", h.ObrisiUslugu)
		r.Get("/troskovi", h.Troskovi)
		r.Get("/troskovi/novi", h.NoviTrosak)
		r.With(doz("artikal.dodaj")).Post("/troskovi/novi", h.SacuvajTrosak)
		r.Get("/troskovi/izmeni/{id}", h.IzmeniTrosak)
		r.With(doz("artikal.izmeni")).Post("/troskovi/izmeni/{id}", h.SacuvajIzmenuTroska)
		r.With(doz("artikal.obrisi")).Post("/troskovi/obrisi/{id}", h.ObrisiTrosak)
		r.Get("/magacin/kartica/{id}", h.MagacinskaKartica)
		r.Get("/magacin/novi", h.NoviArtikal)
		r.With(doz("artikal.dodaj")).Post("/magacin/novi", h.SacuvajArtikal)
		r.Get("/magacin/sledeca-sifra", h.PredlogSifre)
		r.Get("/magacin/izmeni/{id}", h.IzmeniArtikal)
		r.With(doz("artikal.izmeni")).Post("/magacin/izmeni/{id}", h.SacuvajIzmenuArtikla)
		r.With(doz("artikal.obrisi")).Get("/magacin/obrisi/{id}", h.ObrisiArtikal)
		r.With(doz("artikal.obrisi")).Get("/magacin/vrati/{id}", h.VratiArtikal)
		r.With(doz("artikal.izmeni")).Post("/magacin/kartica/{id}/dobavljac/dodaj", h.DodajDobavljacaArtiklu)
		r.With(doz("artikal.izmeni")).Post("/magacin/kartica/{id}/dobavljac/obrisi", h.ObrisiDobavljacaArtikla)
		r.With(doz("artikal.premesti")).Post("/magacin/premesti/{id}", h.PremestiArtikal)
		r.With(doz("artikal.izmeni")).Post("/magacin/promeni-cenu/{id}", h.PromeniCenuArtikla)
		r.With(doz("artikal.izmeni")).Get("/nivelacije", h.Nivelacije)
		r.Get("/magacin/kategorije", h.Kategorije)
		r.With(doz("kategorija.dodaj")).Post("/magacin/kategorije/dodaj", h.DodajKategoriju)
		r.With(doz("kategorija.izmeni")).Post("/magacin/kategorije/izmeni/{id}", h.IzmeniKategoriju)
		r.With(doz("kategorija.obrisi")).Get("/magacin/kategorije/obrisi/{id}", h.ObrisiKategoriju)
		r.With(ntechmw.RequireDozvola(h.DozvoleRepo.ImaDozvolu, "nabavka.pregled")).Get("/nabavke", h.Nabavke)
		r.With(ntechmw.RequireDozvola(h.DozvoleRepo.ImaDozvolu, "nabavka.pregled")).Get("/nabavke/nova", h.NovaNabavka)
		r.With(doz("nabavka.dodaj")).Post("/nabavke/nova", h.SacuvajNabavku)
		r.With(ntechmw.RequireDozvola(h.DozvoleRepo.ImaDozvolu, "nabavka.pregled")).Get("/nabavke/{id}", h.DetaljiNabavke)
		r.With(doz("nabavka.obrisi")).Post("/nabavke/obrisi/{id}", h.ObrisiNabavku)
		r.With(ntechmw.RequireDozvola(h.DozvoleRepo.ImaDozvolu, "dobavljac.pregled")).Get("/dobavljaci", h.Dobavljaci)
		r.Get("/dobavljaci/novi", h.NoviDobavljac)
		r.With(doz("dobavljac.dodaj")).Post("/dobavljaci/novi", h.SacuvajDobavljaca)
		r.Get("/dobavljaci/izmeni/{id}", h.IzmeniDobavljaca)
		r.With(doz("dobavljac.izmeni")).Post("/dobavljaci/izmeni/{id}", h.SacuvajIzmeneDobavljaca)
		r.With(doz("dobavljac.obrisi")).Post("/dobavljaci/obrisi/{id}", h.ObrisiDobavljaca)
		r.With(ntechmw.RequireDozvola(h.DozvoleRepo.ImaDozvolu, "klijent.pregled")).Get("/klijenti", h.Klijenti)
		r.Get("/klijenti/novi", h.NoviKlijent)
		r.With(doz("klijent.dodaj")).Post("/klijenti/novi", h.SacuvajKlijenta)
		r.Get("/klijenti/izmeni/{id}", h.IzmeniKlijenta)
		r.With(doz("klijent.izmeni")).Post("/klijenti/izmeni/{id}", h.SacuvajIzmenuKlijenta)
		r.With(doz("klijent.obrisi")).Post("/klijenti/obrisi/{id}", h.ObrisiKlijenta)
		r.With(ntechmw.RequireDozvola(h.DozvoleRepo.ImaDozvolu, "servis.pregled")).Get("/servis", h.Servis)
		r.With(ntechmw.RequireDozvola(h.DozvoleRepo.ImaDozvolu, "servis.pregled")).Get("/servis/arhiva", h.ArhivaServisa)
		r.Get("/servis/novi", h.NoviNalog)
		r.With(doz("servis.dodaj")).Post("/servis/novi", h.SacuvajNalog)
		r.Get("/servis/izmeni/{id}", h.IzmeniNalog)
		r.With(doz("servis.izmeni")).Post("/servis/izmeni/{id}", h.SacuvajIzmenaNaloga)
		r.With(doz("servis.obrisi")).Post("/servis/obrisi/{id}", h.ObrisiNalog)
		r.With(ntechmw.RequireDozvola(h.DozvoleRepo.ImaDozvolu, "servis.pregled")).Get("/servis/{id}", h.DetaljiNaloga)
		r.With(ntechmw.RequireDozvola(h.DozvoleRepo.ImaDozvolu, "servis.pregled")).Get("/servis/{id}/radni-nalog", h.StampaRadnogNaloga)
		r.With(ntechmw.RequireDozvola(h.DozvoleRepo.ImaDozvolu, "servis.pregled")).Get("/servis/{id}/otpremnica", h.StampaOtpremnice)
		r.With(ntechmw.RequireDozvola(h.DozvoleRepo.ImaDozvolu, "servis.pregled")).Get("/servis/{id}/revers", h.StampaReversa)
		r.With(ntechmw.RequireDozvola(h.DozvoleRepo.ImaDozvolu, "servis.pregled")).Get("/servis/{id}/predracun", h.StampaPredracuna)
		r.With(ntechmw.RequireDozvola(h.DozvoleRepo.ImaDozvolu, "servis.pregled")).Get("/servis/{id}/nalepnica", h.StampaNalepnice)
		r.With(doz("servis.izmeni")).Post("/servis/{id}/status", h.PromeniStatus)
		r.With(doz("servis.izmeni")).Post("/servis/{id}/garancija", h.AzurirajGaranciju)
		r.With(doz("servis.izmeni")).Post("/servis/{id}/garancija-dana", h.AzurirajGarancijaDana)
		r.With(doz("servis.izmeni")).Post("/servis/{id}/predvidjen-datum", h.AzurirajPredvidjenDatum)
		r.With(doz("servis.izmeni")).Post("/servis/{id}/tehnicar", h.AzurirajTehnicar)
		r.With(doz("servis.izmeni")).Post("/servis/{id}/napomena-klijentu", h.SacuvajNapomenuKlijentu)
		r.With(doz("servis.izmeni")).Post("/servis/{id}/nalaz-dijagnostike", h.SacuvajNalazDijagnostike)
		r.With(doz("servis.izmeni")).Post("/servis/{id}/uradjeno", h.SacuvajUradjeno)
		r.With(doz("servis.izmeni")).Post("/servis/{id}/cena-dijagnostike", h.AzurirajCenaDijagnostike)
		r.With(doz("servis.izmeni")).Post("/servis/{id}/odbij-popravku", h.OdbijPopravku)
		r.With(doz("servis.izmeni")).Post("/servis/{id}/delovi", h.DodajDeloNalogu)
		r.With(doz("servis.izmeni")).Post("/servis/{id}/delovi/{deo_id}/obrisi", h.ObrisiDeloNaloga)
		r.With(doz("servis.izmeni")).Post("/servis/{id}/radovi", h.DodajRadNalogu)
		r.With(doz("servis.izmeni")).Post("/servis/{id}/radovi/{rad_id}/obrisi", h.ObrisiRadNaloga)
		r.Get("/izvestaji", h.Izvestaji)
		r.Get("/izvestaji/prometni-list", h.PrometniListMagacina)
		r.Get("/izvestaji/stanje-zaliha", h.StanjeZalihaIzvestaj)
		r.Get("/izvestaji/popis", h.Popis)
		r.With(doz("artikal.izmeni")).Post("/izvestaji/popis", h.SacuvajPopis)
		r.With(ntechmw.RequireDozvola(h.DozvoleRepo.ImaDozvolu, "prodaja.pregled")).Get("/prodaja", h.Prodaja)
		r.Get("/prodaja/nova", h.NovaProdaja)
		r.With(doz("prodaja.dodaj")).Post("/prodaja/nova", h.SacuvajProdaju)
		r.With(doz("prodaja.obrisi")).Post("/prodaja/obrisi/{id}", h.ObrisiProdaju)
		r.With(doz("prodaja.storno")).Post("/prodaja/storno/{id}", h.StornoProdaje)
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
		r.Post("/admin/profil/totp/kodovi", h.AdminTotpRegenerisiKodove)
		r.With(doz("tema.lokalno")).Post("/profil/tema", h.SacuvajLokalnuTemu)
		r.With(doz("tema.lokalno")).Post("/profil/animacija", h.SacuvajLokalnuAnimaciju)
		r.With(doz("tema.lokalno")).Post("/profil/hover", h.SacuvajLokalniHover)
		r.With(doz("tema.lokalno")).Post("/profil/brzina-animacije", h.SacuvajLokalnuBrzinuAnimacije)
		r.Get("/profil/tema", h.ProfilTema)
		r.With(doz("tema.lokalno")).Post("/profil/pozadina", h.ProfilOtpremiPozadinu)
		r.With(doz("tema.lokalno")).Post("/profil/pozadina/ukloni", h.ProfilUkloniPozadinu)
		r.With(doz("tema.lokalno")).Post("/profil/pozadina/stilovi", h.ProfilSacuvajPozadinuStilove)
		r.Post("/profil/avatar", h.ProfilOtpremiAvatar)
		r.Post("/profil/avatar/ukloni", h.ProfilUkloniAvatar)
	})

	slog.Info("NTech pokrenut", "port", port)
	err = http.ListenAndServe(":"+port, r)
	if err != nil {
		slog.Error("port je zauzet ili nije dostupan", "port", port)
		os.Exit(1)
	}
}

// ucitajTotpKljuc vraća 32-bajtni ključ za šifrovanje TOTP tajni. Čita ga iz
// NTECH_TOTP_KEY (base64). Ako env nije postavljen, generiše nov nasumičan ključ
// i dopisuje ga u ntech.env (perm 0600), tako da postojeće instalacije dobiju
// ključ automatski pri prvom pokretanju nove verzije.
//
// VAŽNO: ako se ovaj ključ izgubi ili promeni, postojeće šifrovane TOTP tajne se
// više ne mogu dešifrovati (korisnici moraju ponovo aktivirati 2FA).
func ucitajTotpKljuc(envFajl string) ([]byte, error) {
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
	f, err := os.OpenFile(envFajl, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("otvaranje ntech.env: %w", err)
	}
	defer f.Close()
	if _, err := f.WriteString("\nNTECH_TOTP_KEY=" + enkodiran + "\n"); err != nil {
		return nil, fmt.Errorf("upis ključa u ntech.env: %w", err)
	}
	os.Setenv("NTECH_TOTP_KEY", enkodiran)
	slog.Info("generisan nov NTECH_TOTP_KEY i upisan u ntech.env")
	return kljuc, nil
}

// postaviDemoKorisnika osigurava da pri pokretanju demo instance postoji korisnik "Demo"
// sa ulogom "admin" i resetovanom lozinkom. Poziva se samo kada je NTECH_ENV=demo.
func postaviDemoKorisnika(ctx context.Context, repo db.KorisniciRepository) error {
	const (
		demoIme     = "Demo"
		demoLozinka = "Demo1234"
	)
	hash, err := auth.HashujLozinku(demoLozinka)
	if err != nil {
		return fmt.Errorf("ntech: postaviDemoKorisnika: %w", err)
	}
	korisnik, err := repo.DohvatiPoImenu(ctx, demoIme)
	if err != nil {
		// korisnik ne postoji — kreiraj ga
		if _, err := repo.Kreiraj(ctx, demoIme, hash, "admin"); err != nil {
			return fmt.Errorf("ntech: postaviDemoKorisnika: kreiranje: %w", err)
		}
		slog.Info("demo korisnik kreiran", "korisnik", demoIme)
		return nil
	}
	// korisnik postoji — resetuj lozinku i osiguraj da je aktivan
	if err := repo.PromeniLozinku(ctx, korisnik.ID, hash); err != nil {
		return fmt.Errorf("ntech: postaviDemoKorisnika: lozinka: %w", err)
	}
	if !korisnik.Aktivan {
		if err := repo.AzurirajAktivan(ctx, korisnik.ID, true); err != nil {
			return fmt.Errorf("ntech: postaviDemoKorisnika: aktivan: %w", err)
		}
	}
	slog.Info("demo korisnik resetovan", "korisnik", demoIme)
	return nil
}

// napraviBackup kreira konzistentnu kopiju baze i briše najstarije preko zadatog broja kopija.
// Koristi već otvorenu vezu ka bazi (VACUUM INTO je bezbedan na pooled konekciji).
func napraviBackup(db *sql.DB, putanjaBaze string, maxKopija int) {
	if _, err := os.Stat(putanjaBaze); os.IsNotExist(err) {
		return
	}

	folder := "backups"
	if err := os.MkdirAll(folder, 0755); err != nil {
		slog.Error("backup: ne mogu kreirati folder", "error", err)
		return
	}

	ime := fmt.Sprintf("ntech_%s.db", time.Now().Format("20060102_150405"))
	odrediste := filepath.Join(folder, ime)

	if _, err := db.ExecContext(context.Background(), "VACUUM INTO ?", odrediste); err != nil {
		slog.Error("backup: greška pri pravljenju backup-a", "error", err)
		return
	}

	slog.Info("backup kreiran", "putanja", odrediste)
	ocistiStareBackupe(folder, maxKopija)
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
