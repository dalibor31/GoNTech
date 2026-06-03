package config

import (
	"fmt"
	"net"
	"os"
)

// lista portova koje proveravamo pri prvom pokretanju
var kandidatPortovi = []int{8080, 3000, 8000, 9090}

// proverava da li je port slobodan
func JelPortSlobodan(port int) bool {
	adresa := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", adresa)
	if err != nil {
		return false
	}
	listener.Close()
	return true
}

// vraća prvi slobodan port sa liste kandidata
func NadjiSlobodanPort() int {
	for _, port := range kandidatPortovi {
		if JelPortSlobodan(port) {
			return port
		}
	}
	// ako ni jedan nije slobodan, vraćamo 0
	return 0
}

// proverava da li je ovo prvo pokretanje programa
func JelPrvoPokretanje() bool {
	_, err := os.Stat("ntech.env")
	return os.IsNotExist(err)
}

// PortStatus čuva informaciju o jednom portu
type PortStatus struct {
	Port     int  `json:"port"`
	Slobodan bool `json:"slobodan"`
}

// StatusPortova vraća listu svih kandidat portova sa statusom
func StatusPortova() []PortStatus {
	rezultat := make([]PortStatus, len(kandidatPortovi))
	for i, port := range kandidatPortovi {
		rezultat[i] = PortStatus{
			Port:     port,
			Slobodan: JelPortSlobodan(port),
		}
	}
	return rezultat
}

// SacuvajEnv upisuje izabrani port u ntech.env fajl
func SacuvajEnv(port int) error {
	sadrzaj := fmt.Sprintf("NTECH_PORT=%d\n", port)
	return os.WriteFile("ntech.env", []byte(sadrzaj), 0600)
}
