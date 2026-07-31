package handler

import (
	"testing"

	"ntech/internal/model"
)

// TestKorisnikSmeDaMenjaPodsetnik proverava zaštitu od IDOR-a: radnik sme da menja/briše
// samo sopstveni podsetnik, admin/superadmin smeju bilo koji.
func TestKorisnikSmeDaMenjaPodsetnik(t *testing.T) {
	radnik := &model.Korisnik{ID: 1, Uloga: "radnik"}
	drugiRadnik := &model.Korisnik{ID: 2, Uloga: "radnik"}
	admin := &model.Korisnik{ID: 3, Uloga: "admin"}

	svojPodsetnik := &model.Podsetnik{ID: 100, KorisnikID: p(1)}
	tudjPodsetnik := &model.Podsetnik{ID: 101, KorisnikID: p(2)}
	nedodeljenPodsetnik := &model.Podsetnik{ID: 102, KorisnikID: nil}

	slucajevi := []struct {
		naziv     string
		korisnik  *model.Korisnik
		podsetnik *model.Podsetnik
		ocekivano bool
	}{
		{"radnik menja svoj podsetnik", radnik, svojPodsetnik, true},
		{"radnik ne sme tuđi podsetnik", radnik, tudjPodsetnik, false},
		{"drugi radnik ne sme tuđi podsetnik", drugiRadnik, svojPodsetnik, false},
		{"radnik ne sme nedodeljen podsetnik", radnik, nedodeljenPodsetnik, false},
		{"admin sme tuđi podsetnik", admin, tudjPodsetnik, true},
		{"admin sme nedodeljen podsetnik", admin, nedodeljenPodsetnik, true},
	}

	for _, sc := range slucajevi {
		t.Run(sc.naziv, func(t *testing.T) {
			dobijeno := korisnikSmeDaMenjaPodsetnik(sc.korisnik, sc.podsetnik)
			if dobijeno != sc.ocekivano {
				t.Errorf("korisnikSmeDaMenjaPodsetnik(%+v, %+v) = %v, očekivano %v",
					sc.korisnik, sc.podsetnik, dobijeno, sc.ocekivano)
			}
		})
	}
}
