package handler

import (
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
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
	"izvestaji",
	"kategorije",
	"klijenti", "klijent_forma",
	"magacin", "magacin_forma",
	"nabavke", "nabavka_forma", "nabavka_detalji",
	"podesavanja", "podesavanja_opste", "podesavanja_izgled", "podesavanja_sistem",
	"pdv_stope",
	"pdv_kir", "pdv_kir_forma",
	"podsetnici", "podsetnik_forma",
	"profil_tema",
	"prodaja", "prodaja_detalji", "prodaja_forma",
	"servis", "servis_forma", "servis_detalji",
}

// standalone su šabloni bez base layouta
var standaloneIme = []string{
	"prijava", "setup", "totp_provera", "prodaja_stampa",
}

// sablonskeFunkcije su pomoćne funkcije dostupne u svim šablonima.
// dict gradi mapu iz parova ključ/vrednost — koristi se da se jednom partialu
// prosledi više vrednosti (npr. {{template "x" (dict "ID" .ID "Lista" $.Lista)}}).
var sablonskeFunkcije = template.FuncMap{
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
		t, err := template.ParseFS(fsys, "web/templates/stranice/"+ime+".html")
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
		if tmpl, err = template.ParseFS(h.TemplatesFS, "web/templates/stranice/"+ime+".html"); err != nil {
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
