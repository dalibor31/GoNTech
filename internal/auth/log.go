package auth

import (
	"log"
	"os"
	"path/filepath"
	"time"
)

var authLogger *log.Logger

// InitAuthLog otvara log fajl za auth događaje (pokušaji prijave, zaključavanja)
func InitAuthLog() {
	putanje := []string{"/var/log/ntech/auth.log", "./logs/auth.log"}
	for _, p := range putanje {
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			continue
		}
		f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			continue
		}
		authLogger = log.New(f, "", 0)
		log.Printf("Auth log: %s", p)
		return
	}
	// fallback — logujemo u standardni izlaz
	authLogger = log.New(os.Stdout, "", 0)
}

func zapisiAuth(nivo, ip, korisnik, razlog string) {
	if authLogger == nil {
		return
	}
	authLogger.Printf("%s %s user=%s ip=%s reason=%s",
		time.Now().Format("2006-01-02 15:04:05"), nivo, korisnik, ip, razlog)
}

// LogNeuspehPrijave beleži neuspeli pokušaj prijave u fail2ban-kompatibilnom formatu
func LogNeuspehPrijave(ip, korisnik, razlog string) {
	zapisiAuth("[FAILED]", ip, korisnik, razlog)
}

// LogUspehPrijave beleži uspešnu prijavu
func LogUspehPrijave(ip, korisnik string) {
	zapisiAuth("[SUCCESS]", ip, korisnik, "ok")
}

// LogZaklucano beleži pokušaj prijave sa zaključanog IP-a
func LogZaklucano(ip, korisnik string) {
	zapisiAuth("[LOCKED]", ip, korisnik, "too_many_attempts")
}
