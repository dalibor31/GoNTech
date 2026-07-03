package auth

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/png"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

// HashujLozinku kreira bcrypt heš lozinke
func HashujLozinku(lozinka string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(lozinka), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("ntech: auth.HashujLozinku: %w", err)
	}
	return string(hash), nil
}

// ProveriLozinku proverava da li lozinka odgovara heš vrednosti
func ProveriLozinku(hash, lozinka string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(lozinka)) == nil
}

// dummyHash je bcrypt heš fiksne vrednosti, izračunat jednom pri pokretanju.
// Koristi ga IzjednaciVremeProvere kada korisnik ne postoji.
var dummyHash []byte

func init() {
	h, err := bcrypt.GenerateFromPassword([]byte("ntech-dummy-lozinka"), bcryptCost)
	if err != nil {
		// cost je fiksna konstanta i lozinka nije prazna — ovo se praktično ne
		// može desiti; ako se ipak desi, tiho propadanje bi obesmislilo
		// anti-enumeraciju u IzjednaciVremeProvere, pa je bolje pući na startu.
		panic(fmt.Sprintf("ntech: auth: generisanje dummyHash nije uspelo: %v", err))
	}
	dummyHash = h
}

// IzjednaciVremeProvere izvršava bcrypt poređenje protiv fiksnog heša da bi vreme
// odgovora bilo isto kao kod postojećeg korisnika sa pogrešnom lozinkom —
// time se sprečava enumeracija korisničkih imena merenjem vremena odgovora.
func IzjednaciVremeProvere(lozinka string) {
	_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(lozinka))
}

// GenerisiToken generiše nasumičan UUID token za sesiju
func GenerisiToken() string {
	return uuid.New().String()
}

// TotpPodaci sadrži tajnu, URI i base64-enkodiran QR kod za prikaz
type TotpPodaci struct {
	Tajna    string
	URI      string
	QRBase64 string
}

// GenerisuTotpTajnu generiše novu TOTP tajnu za korisnika
func GenerisuTotpTajnu(korisnickoIme, nazivFirme string) (*TotpPodaci, error) {
	if nazivFirme == "" {
		nazivFirme = "NTech"
	}
	kljuc, err := totp.Generate(totp.GenerateOpts{
		Issuer:      nazivFirme,
		AccountName: korisnickoIme,
	})
	if err != nil {
		return nil, fmt.Errorf("ntech: auth.GenerisuTotpTajnu: %w", err)
	}
	img, err := kljuc.Image(200, 200)
	if err != nil {
		return nil, fmt.Errorf("ntech: auth.GenerisuTotpTajnu: qr: %w", err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("ntech: auth.GenerisuTotpTajnu: png: %w", err)
	}
	return &TotpPodaci{
		Tajna:    kljuc.Secret(),
		URI:      kljuc.URL(),
		QRBase64: base64.StdEncoding.EncodeToString(buf.Bytes()),
	}, nil
}

// RegenerisiTotpQR vraća URI i base64 QR sliku za već postojeću TOTP tajnu
func RegenerisiTotpQR(tajna, korisnickoIme, nazivFirme string) (uri, qrBase64 string) {
	kljuc, err := totp.Generate(totp.GenerateOpts{
		Issuer:      nazivFirme,
		AccountName: korisnickoIme,
		Secret:      []byte(tajna),
	})
	if err != nil {
		return "", ""
	}
	img, err := kljuc.Image(200, 200)
	if err != nil {
		return kljuc.URL(), ""
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return kljuc.URL(), ""
	}
	return kljuc.URL(), base64.StdEncoding.EncodeToString(buf.Bytes())
}

// VerifikujTotpKod proverava TOTP kod korisnika
func VerifikujTotpKod(kod, tajna string) bool {
	return totp.Validate(kod, tajna)
}
