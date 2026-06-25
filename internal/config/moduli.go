package config

// Ovaj fajl izvodi iz profila firme (vidi Project.md §2, §4) koji su zakonski
// moduli uključeni za datu firmu. Profil se čuva kao key-value u tabeli
// `podesavanja`, pa helper prima već učitanu mapu (čista funkcija — lako se
// testira bez baze). Ovo je sloj IZNAD RBAC-a: „da li firma uopšte koristi
// modul", nezavisno od „da li korisnik sme".

// Ključevi profila firme u tabeli `podesavanja`.
const (
	KljucPravniOblik   = "firma_pravni_oblik"
	KljucPdvObveznik   = "firma_pdv_obveznik"
	KljucFiskalizacija = "firma_fiskalizacija"
	KljucRezim         = "firma_rezim"
)

// Nazivi modula koje program pali/gasi prema profilu firme.
const (
	ModulPdv           = "pdv"           // KIR/KPR evidencija, PDV obračun
	ModulFiskalizacija = "fiskalizacija" // Teron L-PFR, fiskalni računi
	ModulKpo           = "kpo"           // Knjiga o ostvarenom prometu (paušalci)
	ModulDvojno        = "dvojno"        // Dvojno knjigovodstvo (DOO, preduzetnici koji ga vode)
)

// ModulUkljucen vraća da li je dati zakonski modul aktivan za firmu, na osnovu
// profila firme. Režim „samo evidencija" gasi ceo zakonski sloj — tada nijedan
// modul nije uključen, bez obzira na ostale prekidače.
func ModulUkljucen(podesavanja map[string]string, modul string) bool {
	// podrazumevano (ključ još ne postoji) je „samo evidencija" — najbezbednije
	// stanje: stara instalacija ne počinje da se ponaša kao poreski obveznik.
	rezim := podesavanja[KljucRezim]
	if rezim == "" {
		rezim = "samo_evidencija"
	}
	if rezim != "pun" {
		return false
	}

	pravniOblik := podesavanja[KljucPravniOblik]

	switch modul {
	case ModulFiskalizacija:
		// Fiskalizacija je obavezna za SVE koji izdaju račune građanima (maloprodaja),
		// bez obzira na pravni oblik. Nezavisan prekidač.
		return podesavanja[KljucFiskalizacija] == "da"

	case ModulPdv:
		// PDV evidencija se vodi kad je firma u sistemu PDV-a (obavezno preko 8M,
		// ili dobrovoljno). Dostupno svim pravnim oblicima.
		return podesavanja[KljucPdvObveznik] == "da"

	case ModulKpo:
		// KPO (knjiga o ostvarenom prometu) vode paušalci i preduzetnici
		// koji vode proste poslovne knjige (ne dvojno).
		return pravniOblik == "pausalac" || pravniOblik == "preduzetnik_knjige"

	case ModulDvojno:
		// Dvojno knjigovodstvo je obavezno za DOO. Preduzetnici koji vode
		// knjige mogu birati prosto ili dvojno — dvojno je opciono
		// (budući toggle: firma_knjigovodstvo = "dvojno").
		return pravniOblik == "doo"

	default:
		return false
	}
}

// SviModuli vraća mapu svih poznatih modula → da li su uključeni za dati profil firme.
// Koristi se da šabloni uslovno prikazuju stavke menija (analogno mapi Dozvole).
func SviModuli(podesavanja map[string]string) map[string]bool {
	moduli := []string{ModulPdv, ModulFiskalizacija, ModulKpo, ModulDvojno}
	m := make(map[string]bool, len(moduli))
	for _, modul := range moduli {
		m[modul] = ModulUkljucen(podesavanja, modul)
	}
	return m
}
