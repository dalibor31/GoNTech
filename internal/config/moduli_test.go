package config

import (
	"maps"
	"testing"
)

func TestModulUkljucen(t *testing.T) {
	// pun je profil punog režima sa svim prekidačima upaljenim i pravnim oblikom doo;
	// pojedinačni testovi ga prepravljaju po potrebi preko pomoćne funkcije izmeni.
	pun := map[string]string{
		KljucRezim:         "pun",
		KljucPdvObveznik:   "da",
		KljucFiskalizacija: "da",
		KljucPravniOblik:   "doo",
	}
	// izmeni vraća kopiju mape sa promenjenim/dodatim ključem (da testovi ne dele stanje)
	izmeni := func(osnova map[string]string, kljuc, vrednost string) map[string]string {
		kopija := make(map[string]string, len(osnova)+1)
		maps.Copy(kopija, osnova)
		kopija[kljuc] = vrednost
		return kopija
	}

	testovi := []struct {
		naziv       string
		podesavanja map[string]string
		modul       string
		ocekivano   bool
	}{
		// master prekidač: režim „samo evidencija" gasi sve
		{"prazna mapa — pdv ugašen (default samo_evidencija)", map[string]string{}, ModulPdv, false},
		{"prazna mapa — fiskalizacija ugašena", map[string]string{}, ModulFiskalizacija, false},
		{"prazna mapa — kpo ugašen", map[string]string{}, ModulKpo, false},
		{"prazna mapa — dvojno ugašeno", map[string]string{}, ModulDvojno, false},
		{"samo evidencija gasi pdv iako je obveznik", izmeni(pun, KljucRezim, "samo_evidencija"), ModulPdv, false},
		{"samo evidencija gasi fiskalizaciju iako je upaljena", izmeni(pun, KljucRezim, "samo_evidencija"), ModulFiskalizacija, false},

		// fiskalizacija — zaseban prekidač, nezavisan od oblika
		{"pun + fiskalizacija da", pun, ModulFiskalizacija, true},
		{"pun + fiskalizacija ne", izmeni(pun, KljucFiskalizacija, "ne"), ModulFiskalizacija, false},

		// pdv — vezan isključivo za prekidač obveznika
		{"pun + obveznik da", pun, ModulPdv, true},
		{"pun + obveznik ne (čak i doo)", izmeni(pun, KljucPdvObveznik, "ne"), ModulPdv, false},

		// kpo — samo paušalac
		{"pun + paušalac → kpo", izmeni(pun, KljucPravniOblik, "pausalac"), ModulKpo, true},
		{"pun + doo → nema kpo", pun, ModulKpo, false},
		{"pun + preduzetnik → nema kpo", izmeni(pun, KljucPravniOblik, "preduzetnik_knjige"), ModulKpo, false},

		// dvojno — samo doo
		{"pun + doo → dvojno", pun, ModulDvojno, true},
		{"pun + paušalac → nema dvojno", izmeni(pun, KljucPravniOblik, "pausalac"), ModulDvojno, false},
		{"pun + preduzetnik → nema dvojno (buduća podela)", izmeni(pun, KljucPravniOblik, "preduzetnik_knjige"), ModulDvojno, false},

		// nepoznat modul
		{"nepoznat modul → false", pun, "nepostojeci", false},
	}

	for _, tt := range testovi {
		t.Run(tt.naziv, func(t *testing.T) {
			if got := ModulUkljucen(tt.podesavanja, tt.modul); got != tt.ocekivano {
				t.Errorf("ModulUkljucen(%q) = %v, očekivano %v", tt.modul, got, tt.ocekivano)
			}
		})
	}
}
