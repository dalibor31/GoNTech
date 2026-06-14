package middleware

// sve poznate akcije u sistemu
// Napomena: pregled magacina i podsetnici su namerno javni (bez dozvole) i nisu ovde.
// Upravljanje korisnicima ide preko uloge (RequireAdmin middleware), ne preko dozvole.
var sveAkcije = []string{
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
	"prodaja.storno",
	"klijent.pregled",
	"klijent.dodaj",
	"klijent.izmeni",
	"klijent.obrisi",
	"izvestaj.pregled",
	"podesavanja.pregled",
	"podesavanja.izmeni",
	"podesavanja.login_pozadina",
	"backup.pregled",
	"backup.pokreni",
	"tema.lokalno",
	"dashboard.prihod",
	"pdv.pregled",
	"pdv.dodaj",
	"pdv.obrisi",
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
		// artikal (pregled magacina je javan — nije dozvola)
		case "artikal.dodaj", "artikal.izmeni",
			"artikal.obrisi", "artikal.premesti":
			return true
		// kategorija
		case "kategorija.pregled", "kategorija.dodaj", "kategorija.izmeni", "kategorija.obrisi":
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
		// izveštaji i podešavanja
		case "izvestaj.pregled",
			"podesavanja.pregled", "podesavanja.izmeni":
			return true
		// backup
		case "backup.pregled", "backup.pokreni":
			return true
		// pozadina prijavne stranice
		case "podesavanja.login_pozadina":
			return true
		// lokalna tema
		case "tema.lokalno":
			return true
		// dashboard — prihod samo admin+
		case "dashboard.prihod":
			return true
		// PDV evidencija (KIR/KPR) — administrativno, radnik nema
		case "pdv.pregled", "pdv.dodaj", "pdv.obrisi":
			return true
		}
		return false

	case "radnik":
		switch akcija {
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
