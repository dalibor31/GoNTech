package handler

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"math"
	"net/http"
	"strings"
)

var bazniSabloni = []string{
	"web/templates/teme/podrazumevana/base.html",
	"web/templates/komponente/sidebar.html",
	"web/templates/komponente/topbar.html",
}

// saSidebar su šabloni koji koriste base layout (sidebar + topbar)
var saSidebar = []string{
	"admin_korisnici", "admin_profil", "admin_login_istorija", "admin_dozvole",
	"dashboard",
	"dobavljaci", "dobavljac_forma",
	"izvestaji", "prometni_list", "stanje_zaliha", "popis",
	"kategorije",
	"klijenti", "klijent_forma",
	"magacin", "magacin_forma", "magacin_kartica",
	"nabavke", "nabavka_forma", "nabavka_detalji",
	"podesavanja", "podesavanja_opste", "podesavanja_izgled", "podesavanja_sistem", "podesavanja_servis", "podesavanja_fiskalizacija",
	"pdv_stope",
	"pdv_kir", "pdv_kir_forma", "pdv_kir_dnevni_pazar",
	"pdv_kpr", "pdv_kpr_forma",
	"pdv_obracun",
	"kpo", "kpo_forma",
	"nivelacije",
	"podsetnici", "podsetnik_forma",
	"profil_tema",
	"prodaja", "prodaja_detalji", "prodaja_forma",
	"servis", "servis_arhiva", "servis_forma", "servis_detalji",
	"usluge", "usluge_forma",
	"troskovi", "troskovi_forma",
}

// standalone su šabloni bez base layouta
var standaloneIme = []string{
	"prijava", "setup", "totp_provera", "prodaja_stampa", "servis_radni_nalog", "servis_otpremnica", "servis_revers", "servis_predracun", "servis_nalepnica", "servis_status_javni", "servis_garantni_list", "servis_eskalacioni_list", "fiskal_verifikacija", "popis_stampa",
}

// sablonskeFunkcije su pomoćne funkcije dostupne u svim šablonima.
// dict gradi mapu iz parova ključ/vrednost — koristi se da se jednom partialu
// prosledi više vrednosti (npr. {{template "x" (dict "ID" .ID "Lista" $.Lista)}}).
var sablonskeFunkcije = template.FuncMap{
	// formatBroj formatira float pointer kao ceo broj (zaokružen) — nil vraca ""
	"formatBroj": func(v *float64) string {
		if v == nil {
			return ""
		}
		return fmt.Sprintf("%d", int64(math.Round(*v)))
	},
	// dinari formatira iznos sa separatorom hiljada (tačka) i 2 decimale (zarez):
	// 1234567.5 → "1.234.567,50"; prihvata float64 i int64
	"dinari": func(v any) string {
		switch val := v.(type) {
		case int64:
			return formatirajDinare(float64(val), 2)
		case int:
			return formatirajDinare(float64(val), 2)
		case float64:
			return formatirajDinare(val, 2)
		default:
			return fmt.Sprintf("%v", v)
		}
	},
	// dinariCeli formatira iznos sa separatorom hiljada, bez decimala: 1234567 → "1.234.567"
	// Prihvata float64 i int64 (PPPDV polja su int64).
	"dinariCeli": func(v any) string {
		switch val := v.(type) {
		case int64:
			return formatirajDinare(float64(val), 0)
		case int:
			return formatirajDinare(float64(val), 0)
		case float64:
			return formatirajDinare(val, 0)
		default:
			return fmt.Sprintf("%v", v)
		}
	},
	// telefon formatira srpski broj telefona radi lakšeg čitanja: "0641234567" → "064 123 4567"
	"telefon": formatirajTelefon,
	// garancijaTekst prikazuje trajanje garancije (dani) na srpskom: 45 → „1 mesec i 15 dana"
	"garancijaTekst": garancijaTekst,
	// json serijalizuje vrednost u JSON za ugradnju u <script> (npr. Alpine x-data);
	// template.JS sprečava da html/template dodatno escapuje rezultat
	"json": func(v any) (template.JS, error) {
		b, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("ntech: json helper: %w", err)
		}
		return template.JS(b), nil
	},
	// statusPre vraća true ako je `a` pre `b` u redosledu statusa
	"statusPre": func(a, b string, statusi []string) bool {
		ia, ib := -1, -1
		for i, s := range statusi {
			if s == a {
				ia = i
			}
			if s == b {
				ib = i
			}
		}
		return ia >= 0 && ib >= 0 && ia < ib
	},
	"dict": func(parovi ...any) (map[string]any, error) {
		if len(parovi)%2 != 0 {
			return nil, fmt.Errorf("dict: neparan broj argumenata")
		}
		m := make(map[string]any, len(parovi)/2)
		for i := 0; i < len(parovi); i += 2 {
			kljuc, ok := parovi[i].(string)
			if !ok {
				return nil, fmt.Errorf("dict: ključ mora biti string")
			}
			m[kljuc] = parovi[i+1]
		}
		return m, nil
	},
	// zbirF64 vraća zbir dva float64 — za aritmetiku u šablonima
	"zbirF64": func(a, b float64) float64 { return a + b },
}

// KreirajKes parsuje sve šablone iz fsys i vraća ih keširane u mapi
func KreirajKes(fsys fs.FS) (map[string]*template.Template, error) {
	kes := make(map[string]*template.Template)

	for _, ime := range saSidebar {
		fajlovi := make([]string, len(bazniSabloni), len(bazniSabloni)+1)
		copy(fajlovi, bazniSabloni)
		fajlovi = append(fajlovi, "web/templates/stranice/"+ime+".html")
		t, err := template.New(ime).Funcs(sablonskeFunkcije).ParseFS(fsys, fajlovi...)
		if err != nil {
			return nil, fmt.Errorf("kes: %s: %w", ime, err)
		}
		kes[ime] = t
	}

	for _, ime := range standaloneIme {
		// ime+".html" mora biti ime roota da bi Execute() pronašlo sadržaj fajla
		t, err := template.New(ime+".html").Funcs(sablonskeFunkcije).ParseFS(fsys, "web/templates/stranice/"+ime+".html")
		if err != nil {
			return nil, fmt.Errorf("kes: %s: %w", ime, err)
		}
		kes[ime] = t
	}

	return kes, nil
}

// renderujTemplate renderuje šablon sa base layoutom
// U produkciji koristi keš; u razvoju parsuje svaki put (hot reload)
func (h *Handler) renderujTemplate(w http.ResponseWriter, ime string, podaci any) {
	// HTML se ne kešira: browser uvek revalidira pa posle nove verzije (npr. Docker
	// deploy) dobije svežu stranicu sa novim AssetV tokenom, koji onda povlači svež
	// CSS/JS. Statika ostaje immutable (URL nosi ?v=verzija).
	w.Header().Set("Cache-Control", "no-cache")
	var tmpl *template.Template

	if h.Templates != nil {
		t, ok := h.Templates[ime]
		if !ok {
			slog.Error("šablon nije pronađen", "ime", ime)
			http.Error(w, "Greška pri učitavanju stranice", http.StatusInternalServerError)
			return
		}
		tmpl = t
	} else {
		fajlovi := make([]string, len(bazniSabloni), len(bazniSabloni)+1)
		copy(fajlovi, bazniSabloni)
		fajlovi = append(fajlovi, "web/templates/stranice/"+ime+".html")
		var err error
		if tmpl, err = template.New(ime).Funcs(sablonskeFunkcije).ParseFS(h.TemplatesFS, fajlovi...); err != nil {
			slog.Error("greška pri parsiranju šablona", "ime", ime, "error", err)
			http.Error(w, "Greška pri učitavanju stranice", http.StatusInternalServerError)
			return
		}
	}

	if err := tmpl.ExecuteTemplate(w, "base", podaci); err != nil {
		slog.Error("greška pri renderovanju šablona", "ime", ime, "error", err)
		http.Error(w, "Greška pri prikazu stranice", http.StatusInternalServerError)
	}
}

// renderujStandalone renderuje šablon bez base layouta (prijava, setup, itd.)
func (h *Handler) renderujStandalone(w http.ResponseWriter, ime string, podaci any) {
	// vidi renderujTemplate: HTML se ne kešira da nova verzija odmah stigne do korisnika
	w.Header().Set("Cache-Control", "no-cache")
	var tmpl *template.Template

	if h.Templates != nil {
		t, ok := h.Templates[ime]
		if !ok {
			slog.Error("standalone šablon nije pronađen", "ime", ime)
			http.Error(w, "Greška pri učitavanju stranice", http.StatusInternalServerError)
			return
		}
		tmpl = t
	} else {
		var err error
		// kao u kreirajKes: root mora biti ime+".html" i moraju biti registrovane šablonske funkcije
		if tmpl, err = template.New(ime+".html").Funcs(sablonskeFunkcije).ParseFS(h.TemplatesFS, "web/templates/stranice/"+ime+".html"); err != nil {
			slog.Error("greška pri parsiranju šablona", "ime", ime, "error", err)
			http.Error(w, "Greška pri učitavanju stranice", http.StatusInternalServerError)
			return
		}
	}

	if err := tmpl.Execute(w, podaci); err != nil {
		slog.Error("greška pri renderovanju šablona", "ime", ime, "error", err)
		http.Error(w, "Greška pri prikazu stranice", http.StatusInternalServerError)
	}
}

// formatirajDinare formatira broj sa tačkom kao separatorom hiljada i zarezom
// za decimale (srpski format). decimale = broj decimalnih mesta (0 ili 2).
func formatirajDinare(v float64, decimale int) string {
	negativan := v < 0
	if negativan {
		v = -v
	}

	var ceoStr, decStr string
	if decimale == 2 {
		// radi u stotinkama da zaokruživanje pravilno prenese (npr. 1234567.999 → 1.234.568,00)
		stotinke := int64(math.Round(v * 100))
		ceoStr = fmt.Sprintf("%d", stotinke/100)
		decStr = fmt.Sprintf("%02d", stotinke%100)
	} else {
		ceoStr = fmt.Sprintf("%d", int64(math.Round(v)))
	}

	// ubaci tačke na svake 3 cifre s desna
	var sb []byte
	n := len(ceoStr)
	for i, c := range ceoStr {
		if i > 0 && (n-i)%3 == 0 {
			sb = append(sb, '.')
		}
		sb = append(sb, byte(c))
	}
	rezultat := string(sb)

	if decimale == 2 {
		rezultat += "," + decStr
	}
	if negativan {
		rezultat = "-" + rezultat
	}
	return rezultat
}

// formatirajTelefon formatira srpski broj telefona radi lakšeg čitanja:
// pozivni broj odvojen kosom crtom, ostatak grupisan crticom.
// Primeri: "0641234567" → "064/123-4567", "+381641234567" → "+381 64/123-4567".
// Ako format nije prepoznat, vraća original.
func formatirajTelefon(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	// izdvoj cifre i zapamti da li je međunarodni (+)
	medjunarodni := strings.HasPrefix(s, "+") || strings.HasPrefix(s, "00")
	var cifre []rune
	for _, c := range s {
		if c >= '0' && c <= '9' {
			cifre = append(cifre, c)
		}
	}
	d := string(cifre)

	// međunarodni srpski prefiks (381): "+381 64/123-4567"
	if medjunarodni {
		d = strings.TrimPrefix(d, "00")
		if strings.HasPrefix(d, "381") {
			ostatak := d[3:] // bez vodeće nule, npr. "641234567"
			if len(ostatak) < 7 || len(ostatak) > 9 {
				return s
			}
			return "+381 " + ostatak[:2] + "/" + grupisiTelefon(ostatak[2:])
		}
		return s // strani broj — ne diramo
	}

	// lokalni format: očekujemo vodeću nulu i 8–10 cifara ukupno
	if !strings.HasPrefix(d, "0") || len(d) < 8 || len(d) > 10 {
		return s
	}
	// pozivni (3 cifre, npr. 064/011) "/" ostatak grupisan crticom
	return d[:3] + "/" + grupisiTelefon(d[3:])
}

// grupisiTelefon deli niz cifara u grupe od po 3 crticom (poslednja može 4) — "1234567" → "123-4567"
func grupisiTelefon(d string) string {
	if len(d) <= 4 {
		return d
	}
	var delovi []string
	delovi = append(delovi, d[:3])
	ostatak := d[3:]
	for len(ostatak) > 4 {
		delovi = append(delovi, ostatak[:3])
		ostatak = ostatak[3:]
	}
	delovi = append(delovi, ostatak)
	return strings.Join(delovi, "-")
}
