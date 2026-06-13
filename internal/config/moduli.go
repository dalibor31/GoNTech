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
	ModulPdv           = "pdv"
	ModulFiskalizacija = "fiskalizacija"
	ModulKpo           = "kpo"
	ModulDvojno        = "dvojno"
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

	switch modul {
	case ModulFiskalizacija:
		// fiskalizacija je nezavisna od pravnog oblika — zaseban prekidač
		// „izdaje li račune građanima" (Project.md §3, napomena *).
		return podesavanja[KljucFiskalizacija] == "da"
	case ModulPdv:
		// PDV evidencija se vodi kad je firma u sistemu PDV-a.
		return podesavanja[KljucPdvObveznik] == "da"
	case ModulKpo:
		// KPO (knjiga o ostvarenom prometu) je samo za paušalce.
		return podesavanja[KljucPravniOblik] == "pausalac"
	case ModulDvojno:
		// dvojno knjigovodstvo je za DOO (za preduzetnike je buduća podela, Project.md §8).
		return podesavanja[KljucPravniOblik] == "doo"
	default:
		return false
	}
}
