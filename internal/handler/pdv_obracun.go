package handler

import (
	"net/http"
	"time"

	"ntech/internal/db/sqlite"
	"ntech/internal/model"
)

// PodaciPdvObracun su podaci za stranicu obračuna PDV za period
type PodaciPdvObracun struct {
	model.PodaciStranice
	Od      string
	Do      string
	KirSume model.PdvKirSume
	KprSume model.PdvKprSume
	Obracun model.PdvObracun
	PPPDV   model.PPPDV
}

// PdvObracunStranica računa obavezu PDV za izabrani period.
// Kada period nije zadat, podrazumevano se uzima tekući mesec.
func (h *Handler) PdvObracunStranica(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.zahtevajDozvolu(w, r, "pdv.pregled"); !ok {
		return
	}
	podesavanja, err := sqlite.DohvatiSvaPodesavanja(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "Greška pri učitavanju podešavanja", http.StatusInternalServerError)
		return
	}

	odStr := r.URL.Query().Get("od")
	doStr := r.URL.Query().Get("do")
	// podrazumevani period: tekući mesec (od prvog do poslednjeg dana)
	if odStr == "" && doStr == "" {
		sada := time.Now()
		prvi := time.Date(sada.Year(), sada.Month(), 1, 0, 0, 0, 0, sada.Location())
		poslednji := prvi.AddDate(0, 1, -1)
		odStr = prvi.Format("2006-01-02")
		doStr = poslednji.Format("2006-01-02")
	}

	od := parsiraDatumOpcionalno(odStr)
	do := parsiraDatumOpcionalno(doStr)

	kirZapisi, err := h.PdvKirRepo.Lista(r.Context(), od, do)
	if err != nil {
		http.Error(w, "Greška pri učitavanju knjige izdatih računa", http.StatusInternalServerError)
		return
	}
	kprZapisi, err := h.PdvKprRepo.Lista(r.Context(), od, do)
	if err != nil {
		http.Error(w, "Greška pri učitavanju knjige primljenih računa", http.StatusInternalServerError)
		return
	}

	kirSume := model.SumirajKir(kirZapisi)
	kprSume := model.SumirajKpr(kprZapisi)

	// razdvajamo KPR na domaće i uvozne — uvoz se u PPPDV mapira u polja 006/106,
	// domaća nabavka u 008/108; obračun obaveze ostaje na ukupnom KPR-u (uvozni PDV je odbitni)
	var kprDomaci, kprUvozni []model.PdvKpr
	for _, z := range kprZapisi {
		if z.Uvoz {
			kprUvozni = append(kprUvozni, z)
		} else {
			kprDomaci = append(kprDomaci, z)
		}
	}
	kprDomaceSume := model.SumirajKpr(kprDomaci)
	kprUvozSume := model.SumirajKpr(kprUvozni)

	ps := h.popuniPodaciStranice(r, podesavanja)
	ps.Stranica = "pdv-obracun"
	ps.NaslovStranice = "PDV obračun"
	h.renderujTemplate(w, "pdv_obracun", PodaciPdvObracun{
		PodaciStranice: ps,
		Od:             odStr,
		Do:             doStr,
		KirSume:        kirSume,
		KprSume:        kprSume,
		Obracun:        model.ObracunajPdv(kirSume, kprSume),
		PPPDV:          model.MapirajPPPDV(kirSume, kprDomaceSume, kprUvozSume),
	})
}
