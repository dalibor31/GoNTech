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
