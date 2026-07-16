package handler

import (
	"net/http"
	"strings"
	"time"

	"ntech/internal/db/sqlite"
	"ntech/internal/middleware"
	"ntech/internal/model"
)

// stavkaBlokiraneIP je jedan red na stranici blokiranih IP adresa, sa
// unapred izračunatim preostalim vremenom zaključavanja (ili "" ako je isteklo).
type stavkaBlokiraneIP struct {
	model.BlokiranaIP
	Preostalo string
}

type podaciBlokiraneIP struct {
	model.PodaciStranice
	Lista []stavkaBlokiraneIP
}

// AdminBlokiraneIP prikazuje IP adrese trenutno zaključane zbog previše neuspelih
// pokušaja prijave, sa mogućnošću ručnog odblokiranja (superadmin).
func (h *Handler) AdminBlokiraneIP(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "bezbednost.odblokiraj"); !ok {
		return
	}
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}
	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "blokirane-ip"
	ps.NaslovStranice = "Blokirane IP adrese"

	od := time.Now().Add(-prozorPrijave)
	blokirane, err := h.PokusajiRepo.ListaBlokiranih(r.Context(), od, maxNeuspehaPrijave)
	if err != nil {
		http.Error(w, "Greška pri učitavanju blokiranih IP adresa", http.StatusInternalServerError)
		return
	}

	lista := make([]stavkaBlokiraneIP, 0, len(blokirane))
	for _, b := range blokirane {
		preostalo, zakljucano := h.preostaloBruteforce(r.Context(), b.IP, od)
		if !zakljucano {
			continue
		}
		lista = append(lista, stavkaBlokiraneIP{BlokiranaIP: b, Preostalo: preostalo})
	}

	h.renderujTemplate(w, "blokirane_ip", podaciBlokiraneIP{PodaciStranice: ps, Lista: lista})
}

// AdminOdblokirajIP briše neuspele pokušaje prijave za datu IP adresu, čime se ona
// odmah oslobađa bruteforce blokade.
func (h *Handler) AdminOdblokirajIP(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "bezbednost.odblokiraj"); !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Greška pri čitanju forme", http.StatusBadRequest)
		return
	}
	ip := strings.TrimSpace(r.FormValue("ip"))
	if ip == "" {
		http.Redirect(w, r, "/admin/blokirane-ip", http.StatusSeeOther)
		return
	}
	if err := h.PokusajiRepo.Odblokiraj(r.Context(), ip); err != nil {
		middleware.SetFlash(w, r, h.DB, "greska", "Odblokiranje nije uspelo.")
		http.Redirect(w, r, "/admin/blokirane-ip", http.StatusSeeOther)
		return
	}
	middleware.SetFlash(w, r, h.DB, "uspeh", "IP adresa "+ip+" je odblokirana.")
	http.Redirect(w, r, "/admin/blokirane-ip", http.StatusSeeOther)
}
