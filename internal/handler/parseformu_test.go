package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"ntech/internal/model"
)

// formZahtev gradi POST zahtev sa form vrednostima i parsira ga
func formZahtev(v url.Values) *http.Request {
	r := httptest.NewRequest("POST", "/", strings.NewReader(v.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_ = r.ParseForm()
	return r
}

func TestParseFormuArtikla(t *testing.T) {
	t.Run("validan", func(t *testing.T) {
		a, greska := parseFormuArtikla(formZahtev(url.Values{
			"naziv": {"Pumpa"}, "kolicina": {"10"}, "prodajna_cena": {"1500.50"},
		}))
		if greska != "" {
			t.Fatalf("neočekivana greška: %q", greska)
		}
		if a.Naziv != "Pumpa" || a.Kolicina != 10 || a.ProdajnaCena != 1500.50 {
			t.Fatalf("pogrešno parsiran artikal: %+v", a)
		}
	})
	t.Run("naziv obavezan", func(t *testing.T) {
		if _, greska := parseFormuArtikla(formZahtev(url.Values{"naziv": {""}})); greska == "" {
			t.Fatal("prazan naziv mora dati grešku")
		}
	})
	t.Run("negativna količina", func(t *testing.T) {
		_, greska := parseFormuArtikla(formZahtev(url.Values{"naziv": {"X"}, "kolicina": {"-5"}}))
		if greska == "" {
			t.Fatal("negativna količina mora dati grešku")
		}
	})
	t.Run("neispravna cena", func(t *testing.T) {
		_, greska := parseFormuArtikla(formZahtev(url.Values{"naziv": {"X"}, "prodajna_cena": {"abc"}}))
		if greska == "" {
			t.Fatal("neispravna cena mora dati grešku")
		}
	})
}

func TestParseFormuKlijenta(t *testing.T) {
	t.Run("fizičko bez imena", func(t *testing.T) {
		_, greska := parseFormuKlijenta(formZahtev(url.Values{"tip": {"fizicko"}}))
		if greska == "" {
			t.Fatal("fizičko lice bez imena mora dati grešku")
		}
	})
	t.Run("pravno bez naziva firme", func(t *testing.T) {
		_, greska := parseFormuKlijenta(formZahtev(url.Values{"tip": {"pravno"}}))
		if greska == "" {
			t.Fatal("pravno lice bez naziva firme mora dati grešku")
		}
	})
	t.Run("neispravan email", func(t *testing.T) {
		_, greska := parseFormuKlijenta(formZahtev(url.Values{
			"tip": {"fizicko"}, "ime": {"Pera"}, "email": {"nije-email"},
		}))
		if greska == "" {
			t.Fatal("email bez @ mora dati grešku")
		}
	})
	t.Run("validno fizičko", func(t *testing.T) {
		k, greska := parseFormuKlijenta(formZahtev(url.Values{
			"tip": {"fizicko"}, "ime": {"Pera"}, "prezime": {"Perić"},
		}))
		if greska != "" {
			t.Fatalf("neočekivana greška: %q", greska)
		}
		if k.Tip != "fizicko" || k.Ime != "Pera" {
			t.Fatalf("pogrešno parsiran klijent: %+v", k)
		}
	})
	t.Run("nepoznat tip → fizičko", func(t *testing.T) {
		k, _ := parseFormuKlijenta(formZahtev(url.Values{"tip": {"vanzemaljac"}, "ime": {"Pera"}}))
		if k.Tip != "fizicko" {
			t.Fatalf("nepoznat tip treba da padne na fizičko, dobijeno %q", k.Tip)
		}
	})
}

func TestParseFormuProdaje(t *testing.T) {
	t.Run("bez stavki", func(t *testing.T) {
		_, _, greska := parseFormuProdaje(formZahtev(url.Values{}))
		if greska == "" {
			t.Fatal("prodaja bez stavki mora dati grešku")
		}
	})
	t.Run("količina nula", func(t *testing.T) {
		_, _, greska := parseFormuProdaje(formZahtev(url.Values{
			"artikal_id[]": {"1"}, "kolicina[]": {"0"}, "cena_po_komadu[]": {"100"},
		}))
		if greska == "" {
			t.Fatal("količina 0 mora dati grešku")
		}
	})
	t.Run("neispravan artikal", func(t *testing.T) {
		_, _, greska := parseFormuProdaje(formZahtev(url.Values{
			"artikal_id[]": {"0"}, "kolicina[]": {"1"}, "cena_po_komadu[]": {"100"},
		}))
		if greska == "" {
			t.Fatal("artikal_id 0 mora dati grešku")
		}
	})
	t.Run("validna stavka", func(t *testing.T) {
		nalog, stavke, greska := parseFormuProdaje(formZahtev(url.Values{
			"artikal_id[]": {"3"}, "kolicina[]": {"2"}, "cena_po_komadu[]": {"250"}, "pdv_stopa[]": {"20"},
		}))
		if greska != "" {
			t.Fatalf("neočekivana greška: %q", greska)
		}
		if len(stavke) != 1 {
			t.Fatalf("očekivana 1 stavka, dobijeno %d", len(stavke))
		}
		s := stavke[0]
		if s.ArtikalID != 3 || s.Kolicina != 2 || s.CenaPoKomadu != 250 || s.PdvStopa != 20 {
			t.Fatalf("pogrešno parsirana stavka: %+v", s)
		}
		if nalog.NacinPlacanja != "gotovina" {
			t.Fatalf("podrazumevani način plaćanja treba da bude gotovina, dobijeno %q", nalog.NacinPlacanja)
		}
	})
	t.Run("nesklad broja stavki", func(t *testing.T) {
		_, _, greska := parseFormuProdaje(formZahtev(url.Values{
			"artikal_id[]": {"1", "2"}, "kolicina[]": {"1"}, "cena_po_komadu[]": {"100"},
		}))
		if greska == "" {
			t.Fatal("nesklad broja stavki mora dati grešku")
		}
	})
}

func TestParseFormuNaloga(t *testing.T) {
	t.Run("uređaj obavezan", func(t *testing.T) {
		_, greska := parseFormuNaloga(formZahtev(url.Values{"opis_kvara": {"ne radi"}}))
		if greska == "" {
			t.Fatal("nedostatak uređaja mora dati grešku")
		}
	})
	t.Run("opis kvara obavezan", func(t *testing.T) {
		_, greska := parseFormuNaloga(formZahtev(url.Values{"uredjaj": {"Laptop"}}))
		if greska == "" {
			t.Fatal("nedostatak opisa kvara mora dati grešku")
		}
	})
	t.Run("validan nalog sa podrazumevanim statusom", func(t *testing.T) {
		nalog, greska := parseFormuNaloga(formZahtev(url.Values{
			"uredjaj": {"Laptop"}, "opis_kvara": {"Ne pali se"},
		}))
		if greska != "" {
			t.Fatalf("neočekivana greška: %q", greska)
		}
		if nalog.Status != model.StatusPrimljeno {
			t.Fatalf("podrazumevani status treba da bude %q, dobijeno %q", model.StatusPrimljeno, nalog.Status)
		}
	})
}
