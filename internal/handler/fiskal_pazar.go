package handler

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"ntech/internal/config"
	ntechsqlite "ntech/internal/db/sqlite"
	"ntech/internal/fiskal"
	"ntech/internal/middleware"
	"ntech/internal/model"
)

// contentDispositionZaFajl gradi Content-Disposition header za dati naziv fajla.
// HTTP header vrednosti moraju biti ASCII — naziv sa slovima kao što su Š/Č/Ž
// (npr. iz PFR-a: "DNEVNI IZVEŠTAJ - ...pdf") bi inače napravio nevalidan header
// koji pregledač ne može da parsira. RFC 6266 filename* nosi pravo UTF-8 ime,
// dok filename (ASCII) služi kao fallback za starije klijente.
func contentDispositionZaFajl(ime string) string {
	ascii := strings.Map(func(r rune) rune {
		if r > unicode.MaxASCII {
			return '_'
		}
		return r
	}, ime)
	return fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, ascii, url.PathEscape(ime))
}

// PodaciFiskalniPazar su podaci za stranicu dnevnog fiskalnog preseka (pazara).
type PodaciFiskalniPazar struct {
	model.PodaciStranice
	Presek        *fiskal.FinancialSummary
	Greska        string
	FiskalPodesen bool   // true: pfr_url je podešen
	Danas         string // YYYY-MM-DD, podrazumevani datum za izveštaj (od=do=danas)
}

// FiskalniPazar prikazuje trenutni promet od poslednjeg preseka stanja
// (GET /api/financial/summary na PFR-u) — read-only, ne menja stanje.
func (h *Handler) FiskalniPazar(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "fiskal.pazar"); !ok {
		return
	}
	podesavanja, err := ntechsqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}
	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "fiskal-pazar"
	ps.NaslovStranice = "Dnevni fiskalni presek"

	podaci := PodaciFiskalniPazar{PodaciStranice: ps, Danas: time.Now().Format("2006-01-02")}

	if !h.modulUkljucen(r.Context(), config.ModulFiskalizacija) {
		podaci.Greska = "Fiskalizacija nije uključena u profilu firme — Podešavanja → Opšte → Pravni i poreski status."
		h.renderujTemplate(w, "fiskal_pazar", podaci)
		return
	}

	klijent := h.fiskalKlijent()
	if klijent == nil {
		podaci.Greska = "Fiskalizacija nije podešena — unesi URL PFR servera u Podešavanja → Fiskalizacija."
		h.renderujTemplate(w, "fiskal_pazar", podaci)
		return
	}
	podaci.FiskalPodesen = true

	presek, err := klijent.Presek(r.Context())
	if err != nil {
		podaci.Greska = "Fiskalni server nije dostupan — proveri da li je PFR uređaj/mock pokrenut."
	} else {
		podaci.Presek = presek
	}
	h.renderujTemplate(w, "fiskal_pazar", podaci)
}

// ZakljuciFiskalniDan svodi promet PFR-a na nulu (DELETE /api/financial/summary).
// OPCIONA radnja — samo resetuje brojače na ekranu „Promet od poslednjeg preseka";
// nije zakonska obaveza (za razliku od starih fiskalnih kasa, novi sistem izveštava
// SUF kontinuirano po računu). Pravi dnevni/periodični izveštaj (FiskalniIzvestaj)
// radi nezavisno preko datumskog opsega, bez obzira na to da li je ovo ikad urađeno.
func (h *Handler) ZakljuciFiskalniDan(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "fiskal.zakljucenje"); !ok {
		return
	}
	if !h.modulUkljucen(r.Context(), config.ModulFiskalizacija) {
		middleware.SetFlash(w, r, h.DB, "greska", "Fiskalizacija nije uključena u profilu firme.")
		http.Redirect(w, r, "/fiskal/pazar", http.StatusSeeOther)
		return
	}
	klijent := h.fiskalKlijent()
	if klijent == nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Fiskalizacija nije podešena.")
		http.Redirect(w, r, "/fiskal/pazar", http.StatusSeeOther)
		return
	}
	if err := klijent.ZakljuciDan(r.Context()); err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Presek stanja nije uspeo — fiskalni server nije dostupan.")
		http.Redirect(w, r, "/fiskal/pazar", http.StatusSeeOther)
		return
	}
	middleware.SetFlash(w, r, h.DB, "uspeh", "Brojači prometa na ekranu su resetovani.")
	http.Redirect(w, r, "/fiskal/pazar", http.StatusSeeOther)
}

// FiskalniIzvestaj traži od PFR-a dnevni/periodični izveštaj (POST
// /api/financial/report/summary) i vraća ga korisniku kao fajl za preuzimanje.
// FromDate/ToDate dolaze iz forme (podrazumevano danas-danas) — izveštaj radi
// nezavisno od preseka stanja, pa je tačan bez obzira na to da li je Zaključi dan
// ikad korišćeno.
func (h *Handler) FiskalniIzvestaj(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "fiskal.pazar"); !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}
	if !h.modulUkljucen(r.Context(), config.ModulFiskalizacija) {
		middleware.SetFlash(w, r, h.DB, "greska", "Fiskalizacija nije uključena u profilu firme.")
		http.Redirect(w, r, "/fiskal/pazar", http.StatusSeeOther)
		return
	}
	klijent := h.fiskalKlijent()
	if klijent == nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Fiskalizacija nije podešena.")
		http.Redirect(w, r, "/fiskal/pazar", http.StatusSeeOther)
		return
	}

	zahtev := fiskal.ReportSummaryZahtev{
		FromDate: strings.TrimSpace(r.FormValue("od_datuma")),
		ToDate:   strings.TrimSpace(r.FormValue("do_datuma")),
	}
	odgovor, err := klijent.GenerisiIzvestaj(r.Context(), zahtev)
	if err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Generisanje izveštaja nije uspelo — fiskalni server nije dostupan.")
		http.Redirect(w, r, "/fiskal/pazar", http.StatusSeeOther)
		return
	}

	sadrzaj, err := base64.StdEncoding.DecodeString(odgovor.ReportPdfBase64)
	if err != nil {
		http.Error(w, "Neispravan sadržaj izveštaja", http.StatusBadGateway)
		return
	}

	imeFajla := odgovor.Filename
	if imeFajla == "" {
		imeFajla = "dnevni-izvestaj.pdf"
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", contentDispositionZaFajl(imeFajla))
	w.Write(sadrzaj)
}
