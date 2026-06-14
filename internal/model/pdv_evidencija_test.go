package model

import (
	"math"
	"testing"
	"time"
)

func blizu(a, b float64) bool { return math.Abs(a-b) < 0.01 }

func TestKirIzProdaje(t *testing.T) {
	nalog := ProdajniNalog{ID: 5, BrojNaloga: "P-1", Datum: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)}
	stavke := []StavkaProdaje{
		// 20%: 2 × 120 = 240 (osnovica 200, PDV 40)
		{Kolicina: 2, CenaPoKomadu: 120, PdvStopa: 20},
		// 10%: 1 × 110 = 110 (osnovica 100, PDV 10)
		{Kolicina: 1, CenaPoKomadu: 110, PdvStopa: 10},
		// 0%: 1 × 50 = 50 (oslobođeno, bez PDV)
		{Kolicina: 1, CenaPoKomadu: 50, PdvStopa: 0},
	}

	k := KirIzProdaje(nalog, stavke, "Kupac doo", "123456789", "Niš")

	if k.Izvor != "prodaja" || k.IzvorID == nil || *k.IzvorID != 5 {
		t.Errorf("izvor=%q izvor_id=%v, očekivano prodaja/5", k.Izvor, k.IzvorID)
	}
	if k.BrojDokumenta != "P-1" || k.KupacNaziv != "Kupac doo" || k.KupacPib != "123456789" {
		t.Errorf("zaglavlje ne odgovara: %+v", k)
	}
	if !blizu(k.OsnovicaOpsta, 200) || !blizu(k.PdvOpsta, 40) {
		t.Errorf("opšta: osnovica=%v pdv=%v, očekivano 200/40", k.OsnovicaOpsta, k.PdvOpsta)
	}
	if !blizu(k.OsnovicaPosebna, 100) || !blizu(k.PdvPosebna, 10) {
		t.Errorf("posebna: osnovica=%v pdv=%v, očekivano 100/10", k.OsnovicaPosebna, k.PdvPosebna)
	}
	if !blizu(k.OslobodenSaPravom, 50) {
		t.Errorf("oslobođeno=%v, očekivano 50", k.OslobodenSaPravom)
	}
	if !blizu(k.Ukupno, 400) {
		t.Errorf("ukupno=%v, očekivano 400 (240+110+50)", k.Ukupno)
	}
}

func TestKprIzNabavke(t *testing.T) {
	nabavka := Nabavka{ID: 3, Napomena: "test", Datum: time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)}
	stavke := []NabavkaStavkaPdv{
		{Osnovica: 200, PdvStopa: 20}, // PDV 40
		{Osnovica: 100, PdvStopa: 10}, // PDV 10
		{Osnovica: 50, PdvStopa: 0},   // oslobođena nabavka
	}

	k := KprIzNabavke(nabavka, "Dobavljač doo", "987654321", "Beograd", stavke)

	if k.Izvor != "nabavka" || k.IzvorID == nil || *k.IzvorID != 3 {
		t.Errorf("izvor=%q izvor_id=%v, očekivano nabavka/3", k.Izvor, k.IzvorID)
	}
	if k.BrojDokumenta != "NAB-3" || k.DobavljacNaziv != "Dobavljač doo" || k.DobavljacPib != "987654321" {
		t.Errorf("zaglavlje ne odgovara: %+v", k)
	}
	if !blizu(k.OsnovicaOpsta, 200) || !blizu(k.PdvOpsta, 40) {
		t.Errorf("opšta: osnovica=%v pdv=%v, očekivano 200/40", k.OsnovicaOpsta, k.PdvOpsta)
	}
	if !blizu(k.OsnovicaPosebna, 100) || !blizu(k.PdvPosebna, 10) {
		t.Errorf("posebna: osnovica=%v pdv=%v, očekivano 100/10", k.OsnovicaPosebna, k.PdvPosebna)
	}
	if !blizu(k.OslobodenNabavka, 50) {
		t.Errorf("oslobođena nabavka=%v, očekivano 50", k.OslobodenNabavka)
	}
	if !blizu(k.Ukupno, 400) {
		t.Errorf("ukupno=%v, očekivano 400 (240+110+50)", k.Ukupno)
	}
}

func TestObracunajPdv(t *testing.T) {
	// izlazni: 200 (opšta) + 50 (posebna) = 250; odbitni: 80 + 20 = 100; obaveza = 150
	kir := PdvKirSume{PdvOpsta: 200, PdvPosebna: 50}
	kpr := PdvKprSume{PdvOpsta: 80, PdvPosebna: 20, PdvBezOdbitka: 30}

	o := ObracunajPdv(kir, kpr)

	if !blizu(o.IzlazniPdvUkupno, 250) {
		t.Errorf("izlazni=%v, očekivano 250", o.IzlazniPdvUkupno)
	}
	// PdvBezOdbitka (30) ne sme da uđe u odbitni PDV
	if !blizu(o.OdbitniPdvUkupno, 100) {
		t.Errorf("odbitni=%v, očekivano 100 (bez PdvBezOdbitka)", o.OdbitniPdvUkupno)
	}
	if !blizu(o.Obaveza, 150) || !o.ZaUplatu() {
		t.Errorf("obaveza=%v ZaUplatu=%v, očekivano 150/true", o.Obaveza, o.ZaUplatu())
	}

	// kada je odbitni veći od izlaznog → negativna obaveza (povraćaj/prenos), ZaUplatu=false
	o2 := ObracunajPdv(PdvKirSume{PdvOpsta: 40}, PdvKprSume{PdvOpsta: 100})
	if !blizu(o2.Obaveza, -60) || o2.ZaUplatu() {
		t.Errorf("obaveza=%v ZaUplatu=%v, očekivano -60/false", o2.Obaveza, o2.ZaUplatu())
	}
}

func TestMapirajPPPDV(t *testing.T) {
	kir := PdvKirSume{
		OsnovicaOpsta: 1000.4, PdvOpsta: 200.08, // 003=1000, 103=200
		OsnovicaPosebna: 500.5, PdvPosebna: 50.05, // 004=501 (round), 104=50
		OslobodenSaPravom: 300.2, OslobodenBezPrava: 100.9, // 001=300, 002=101
	}
	kpr := PdvKprSume{
		OsnovicaOpsta: 400, PdvOpsta: 80,
		OsnovicaPosebna: 100, PdvPosebna: 10,
		PdvBezOdbitka: 25, // ne sme da uđe nigde u PPPDV
	}

	p := MapirajPPPDV(kir, kpr)

	if p.Polje001 != 300 || p.Polje002 != 101 || p.Polje003 != 1000 || p.Polje103 != 200 {
		t.Errorf("I deo (001/002/003/103) = %d/%d/%d/%d", p.Polje001, p.Polje002, p.Polje003, p.Polje103)
	}
	if p.Polje004 != 501 || p.Polje104 != 50 {
		t.Errorf("posebna 004/104 = %d/%d, očekivano 501/50", p.Polje004, p.Polje104)
	}
	// 005 = 300+101+1000+501 = 1902; 105 = 200+50 = 250
	if p.Polje005 != 1902 || p.Polje105 != 250 {
		t.Errorf("zbir 005/105 = %d/%d, očekivano 1902/250", p.Polje005, p.Polje105)
	}
	// uvoz i poljoprivrednik se ne prate
	if p.Polje006 != 0 || p.Polje106 != 0 || p.Polje007 != 0 || p.Polje107 != 0 {
		t.Errorf("006/106/007/107 moraju biti 0")
	}
	// 008 = 400+100 = 500; 108 = 80+10 = 90 (PdvBezOdbitka 25 izostavljen)
	if p.Polje008 != 500 || p.Polje108 != 90 {
		t.Errorf("ostali prethodni 008/108 = %d/%d, očekivano 500/90", p.Polje008, p.Polje108)
	}
	if p.Polje009 != 500 || p.Polje109 != 90 {
		t.Errorf("zbir prethodnog 009/109 = %d/%d, očekivano 500/90", p.Polje009, p.Polje109)
	}
	// 110 = 250 − 90 = 160 (za uplatu)
	if p.Polje110 != 160 || p.Povracaj {
		t.Errorf("110=%d Povracaj=%v, očekivano 160/false", p.Polje110, p.Povracaj)
	}

	// scenario povraćaja: izlazni < odbitni → 110 negativan, Povracaj=true
	p2 := MapirajPPPDV(PdvKirSume{PdvOpsta: 30}, PdvKprSume{PdvOpsta: 90})
	if p2.Polje110 != -60 || !p2.Povracaj || p2.Polje110Apsolutno() != 60 {
		t.Errorf("povraćaj: 110=%d Povracaj=%v abs=%d, očekivano -60/true/60", p2.Polje110, p2.Povracaj, p2.Polje110Apsolutno())
	}
}
