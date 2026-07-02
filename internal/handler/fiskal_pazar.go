package handler

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	ntechsqlite "ntech/internal/db/sqlite"
	"ntech/internal/fiskal"
	"ntech/internal/middleware"
	"ntech/internal/model"
)

// PodaciFiskalniPazar su podaci za stranicu dnevnog fiskalnog preseka (pazara).
type PodaciFiskalniPazar struct {
	model.PodaciStranice
	Presek        *fiskal.FinancialSummary
	Greska        string
	FiskalPodesen bool // true: pfr_url je podešen
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

	podaci := PodaciFiskalniPazar{PodaciStranice: ps}

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
// Nepovratna akcija — traži potvrdu na strani korisnika (confirm() u šablonu).
func (h *Handler) ZakljuciFiskalniDan(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "fiskal.zakljucenje"); !ok {
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
	middleware.SetFlash(w, r, h.DB, "uspeh", "Dan je zaključen — presek stanja urađen, brojači prometa resetovani.")
	http.Redirect(w, r, "/fiskal/pazar", http.StatusSeeOther)
}

// FiskalniIzvestaj traži od PFR-a dnevni/periodični izveštaj (POST
// /api/financial/report/summary) i vraća ga korisniku kao fajl za preuzimanje.
// Bez fromDate/toDate parametara izveštaj pokriva period od poslednjeg preseka stanja.
func (h *Handler) FiskalniIzvestaj(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "fiskal.pazar"); !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
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
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, imeFajla))
	w.Write(sadrzaj)
}
