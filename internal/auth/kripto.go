package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// DuzinaTotpKljuca je obavezna dužina ključa za AES-256 (32 bajta).
const DuzinaTotpKljuca = 32

// Sifruj šifruje tekst pomoću AES-256-GCM i vraća base64 zapis u kome je nonce
// zalepljen ispred šifrata i autentifikacionog taga. Ključ mora biti tačno 32
// bajta. Koristi se za TOTP tajne u mirovanju (šifrovane u bazi).
func Sifruj(tekst string, kljuc []byte) (string, error) {
	if len(kljuc) != DuzinaTotpKljuca {
		return "", fmt.Errorf("ntech: auth.Sifruj: ključ mora imati %d bajta", DuzinaTotpKljuca)
	}
	blok, err := aes.NewCipher(kljuc)
	if err != nil {
		return "", fmt.Errorf("ntech: auth.Sifruj: %w", err)
	}
	gcm, err := cipher.NewGCM(blok)
	if err != nil {
		return "", fmt.Errorf("ntech: auth.Sifruj: %w", err)
	}
	// svaka enkripcija dobija nasumičan nonce; on se čuva uz šifrat radi dešifrovanja
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("ntech: auth.Sifruj: %w", err)
	}
	// Seal dodaje šifrat i tag iza nonce-a (prvi argument je „prefiks" rezultata)
	zapis := gcm.Seal(nonce, nonce, []byte(tekst), nil)
	return base64.StdEncoding.EncodeToString(zapis), nil
}

// Desifruj dešifruje zapis koji je napravio Sifruj. Vraća grešku ako zapis nije
// ispravan base64, ako je prekratak, ili ako provera integriteta (GCM tag) ne
// prođe. Ta greška se namerno koristi i za prepoznavanje starih, nešifrovanih
// tajni — pozivalac u tom slučaju tretira vrednost kao već čist tekst.
func Desifruj(zapis string, kljuc []byte) (string, error) {
	if len(kljuc) != DuzinaTotpKljuca {
		return "", fmt.Errorf("ntech: auth.Desifruj: ključ mora imati %d bajta", DuzinaTotpKljuca)
	}
	podaci, err := base64.StdEncoding.DecodeString(zapis)
	if err != nil {
		return "", fmt.Errorf("ntech: auth.Desifruj: %w", err)
	}
	blok, err := aes.NewCipher(kljuc)
	if err != nil {
		return "", fmt.Errorf("ntech: auth.Desifruj: %w", err)
	}
	gcm, err := cipher.NewGCM(blok)
	if err != nil {
		return "", fmt.Errorf("ntech: auth.Desifruj: %w", err)
	}
	if len(podaci) < gcm.NonceSize() {
		return "", errors.New("ntech: auth.Desifruj: zapis je prekratak")
	}
	nonce, sifrat := podaci[:gcm.NonceSize()], podaci[gcm.NonceSize():]
	otvoren, err := gcm.Open(nil, nonce, sifrat, nil)
	if err != nil {
		return "", fmt.Errorf("ntech: auth.Desifruj: %w", err)
	}
	return string(otvoren), nil
}
