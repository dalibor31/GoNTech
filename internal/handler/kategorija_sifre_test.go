package handler

import (
	"testing"

	"ntech/internal/model"
)

func TestIzracunajPlanUskladjivanjaSifri_PopunjavaPrazninu(t *testing.T) {
	kategorije := []model.Kategorija{{ID: 1, Naziv: "Kompjuteri", Kod: "KOMP"}}
	artikli := []model.ArtikalSaKategorijom{
		{Artikal: model.Artikal{ID: 1, KategorijaID: p(1), Sifra: "KOMP-0001"}, KategorijaNaziv: "Kompjuteri"},
		{Artikal: model.Artikal{ID: 2, KategorijaID: p(1), Sifra: "KOMP-0003"}, KategorijaNaziv: "Kompjuteri"},
		{Artikal: model.Artikal{ID: 3, KategorijaID: p(1), Sifra: "STARA-ŠIFRA"}, KategorijaNaziv: "Kompjuteri"},
	}

	plan := izracunajPlanUskladjivanjaSifri(artikli, kategorije)

	if len(plan) != 1 {
		t.Fatalf("plan = %d stavki, očekivano 1", len(plan))
	}
	if plan[0].ArtikalID != 3 || plan[0].NovaSifra != "KOMP-0002" {
		t.Errorf("plan[0] = %+v, očekivano ArtikalID=3 NovaSifra=KOMP-0002", plan[0])
	}
}

func TestIzracunajPlanUskladjivanjaSifri_KategorijaBezKodaSePreskace(t *testing.T) {
	kategorije := []model.Kategorija{{ID: 1, Naziv: "Ram memorije", Kod: ""}}
	artikli := []model.ArtikalSaKategorijom{
		{Artikal: model.Artikal{ID: 1, KategorijaID: p(1), Sifra: "NASUMICNO"}, KategorijaNaziv: "Ram memorije"},
	}

	plan := izracunajPlanUskladjivanjaSifri(artikli, kategorije)

	if len(plan) != 0 {
		t.Fatalf("plan = %d stavki, očekivano 0 (kategorija bez koda)", len(plan))
	}
}

func TestIzracunajPlanUskladjivanjaSifri_VecIspravnaSifraSePreskace(t *testing.T) {
	kategorije := []model.Kategorija{{ID: 1, Naziv: "Kompjuteri", Kod: "KOMP"}}
	artikli := []model.ArtikalSaKategorijom{
		{Artikal: model.Artikal{ID: 1, KategorijaID: p(1), Sifra: "KOMP-0001"}, KategorijaNaziv: "Kompjuteri"},
	}

	plan := izracunajPlanUskladjivanjaSifri(artikli, kategorije)

	if len(plan) != 0 {
		t.Fatalf("plan = %d stavki, očekivano 0 (šifra već ispravna)", len(plan))
	}
}

func TestIzracunajPlanUskladjivanjaSifri_BezKategorijeIdeUnderArt(t *testing.T) {
	kategorije := []model.Kategorija{}
	artikli := []model.ArtikalSaKategorijom{
		{Artikal: model.Artikal{ID: 1, KategorijaID: nil, Sifra: "ART-0001"}},
		{Artikal: model.Artikal{ID: 2, KategorijaID: nil, Sifra: "NEBITNO"}},
	}

	plan := izracunajPlanUskladjivanjaSifri(artikli, kategorije)

	if len(plan) != 1 {
		t.Fatalf("plan = %d stavki, očekivano 1", len(plan))
	}
	if plan[0].ArtikalID != 2 || plan[0].NovaSifra != "ART-0002" {
		t.Errorf("plan[0] = %+v, očekivano ArtikalID=2 NovaSifra=ART-0002", plan[0])
	}
}

func TestIzracunajPlanUskladjivanjaSifri_RedosledPoID(t *testing.T) {
	kategorije := []model.Kategorija{{ID: 1, Naziv: "Kompjuteri", Kod: "KOMP"}}
	artikli := []model.ArtikalSaKategorijom{
		{Artikal: model.Artikal{ID: 5, KategorijaID: p(1), Sifra: "X"}, KategorijaNaziv: "Kompjuteri"},
		{Artikal: model.Artikal{ID: 2, KategorijaID: p(1), Sifra: "Y"}, KategorijaNaziv: "Kompjuteri"},
	}

	plan := izracunajPlanUskladjivanjaSifri(artikli, kategorije)

	if len(plan) != 2 {
		t.Fatalf("plan = %d stavki, očekivano 2", len(plan))
	}
	// ulaz je namerno dat u redosledu ID 5 pa 2 — funkcija sama sortira po ID-u
	// pre dodele brojeva, pa 2 (manji ID) mora dobiti prvi (manji) broj
	if plan[0].ArtikalID != 2 || plan[0].NovaSifra != "KOMP-0001" {
		t.Errorf("plan[0] = %+v, očekivano ArtikalID=2 NovaSifra=KOMP-0001", plan[0])
	}
	if plan[1].ArtikalID != 5 || plan[1].NovaSifra != "KOMP-0002" {
		t.Errorf("plan[1] = %+v, očekivano ArtikalID=5 NovaSifra=KOMP-0002", plan[1])
	}
}

func p(id int64) *int64 { return &id }
