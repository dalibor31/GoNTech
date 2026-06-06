package middleware

// sve poznate akcije u sistemu
var sveAkcije = []string{
	"artikal.pregled",
	"artikal.dodaj",
	"artikal.izmeni",
	"artikal.obrisi",
	"artikal.premesti",
	"kategorija.pregled",
	"kategorija.dodaj",
	"kategorija.izmeni",
	"kategorija.obrisi",
	"nabavka.pregled",
	"nabavka.dodaj",
	"nabavka.obrisi",
	"dobavljac.pregled",
	"dobavljac.dodaj",
	"dobavljac.izmeni",
	"dobavljac.obrisi",
	"servis.pregled",
	"servis.dodaj",
	"servis.izmeni",
	"servis.obrisi",
	"prodaja.pregled",
	"prodaja.dodaj",
	"prodaja.obrisi",
	"klijent.pregled",
	"klijent.dodaj",
	"klijent.izmeni",
	"klijent.obrisi",
	"podsetnik.pregled",
	"podsetnik.dodaj",
	"podsetnik.izmeni",
	"podsetnik.obrisi",
	"izvestaj.pregled",
	"podesavanja.pregled",
	"podesavanja.izmeni",
	"korisnik.pregled",
	"korisnik.dodaj",
	"korisnik.izmeni",
	"korisnik.obrisi",
	"korisnik.uloga",
	"backup.pregled",
	"backup.pokreni",
	"podesavanja.login_pozadina",
	"podesavanja.app_pozadina",
	"tema.globalno",
	"tema.lokalno",
	"dashboard.prihod",
	"prodaja.storno",
}

// SveAkcije vraća listu svih poznatih akcija — koristi se pri inicijalizaciji baze i resetu
func SveAkcije() []string {
	return sveAkcije
}

// ImaDozvolu proverava da li data uloga sme da izvrši traženu akciju
func ImaDozvolu(uloga, akcija string) bool {
	switch uloga {
	case "superadmin":
		// superadmin sme sve
		return true

	case "admin":
		switch akcija {
		// artikal
		case "artikal.pregled", "artikal.dodaj", "artikal.izmeni",
			"artikal.obrisi", "artikal.premesti":
			return true
		// kategorija
		case "kategorija.pregled", "kategorija.dodaj",
			"kategorija.izmeni", "kategorija.obrisi":
			return true
		// nabavka
		case "nabavka.pregled", "nabavka.dodaj", "nabavka.obrisi":
			return true
		// dobavljač
		case "dobavljac.pregled", "dobavljac.dodaj",
			"dobavljac.izmeni", "dobavljac.obrisi":
			return true
		// servis
		case "servis.pregled", "servis.dodaj",
			"servis.izmeni", "servis.obrisi":
			return true
		// prodaja
		case "prodaja.pregled", "prodaja.dodaj", "prodaja.obrisi", "prodaja.storno":
			return true
		// klijent
		case "klijent.pregled", "klijent.dodaj",
			"klijent.izmeni", "klijent.obrisi":
			return true
		// podsetnik
		case "podsetnik.pregled", "podsetnik.dodaj",
			"podsetnik.izmeni", "podsetnik.obrisi":
			return true
		// izveštaji i podešavanja
		case "izvestaj.pregled",
			"podesavanja.pregled", "podesavanja.izmeni":
			return true
		// korisnici (bez promene uloge)
		case "korisnik.pregled", "korisnik.dodaj",
			"korisnik.izmeni", "korisnik.obrisi":
			return true
		// backup
		case "backup.pregled", "backup.pokreni":
			return true
		// pozadinske slike
		case "podesavanja.login_pozadina", "podesavanja.app_pozadina":
			return true
		// teme
		case "tema.globalno", "tema.lokalno":
			return true
		// dashboard — prihod samo admin+
		case "dashboard.prihod":
			return true
		}
		return false

	case "radnik":
		switch akcija {
		// artikal — samo pregled
		case "artikal.pregled":
			return true
		// kategorija — samo pregled
		case "kategorija.pregled":
			return true
		// dobavljač — bez brisanja
		case "dobavljac.pregled", "dobavljac.dodaj", "dobavljac.izmeni":
			return true
		// servis — bez brisanja
		case "servis.pregled", "servis.dodaj", "servis.izmeni":
			return true
		// prodaja — bez brisanja i storna
		case "prodaja.pregled", "prodaja.dodaj":
			return true
		// klijent — bez brisanja
		case "klijent.pregled", "klijent.dodaj", "klijent.izmeni":
			return true
		// podsetnik — sve
		case "podsetnik.pregled", "podsetnik.dodaj",
			"podsetnik.izmeni", "podsetnik.obrisi":
			return true
		// lokalna tema
		case "tema.lokalno":
			return true
		}
		return false
	}

	return false
}

// SveDozvole vraća mapu svih akcija sa vrednošću true/false za datu ulogu
func SveDozvole(uloga string) map[string]bool {
	m := make(map[string]bool, len(sveAkcije))
	for _, akcija := range sveAkcije {
		m[akcija] = ImaDozvolu(uloga, akcija)
	}
	return m
}
