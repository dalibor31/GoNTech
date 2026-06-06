package handler

import (
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
)

var bazniSabloni = []string{
	"web/templates/teme/podrazumevana/base.html",
	"web/templates/komponente/sidebar.html",
	"web/templates/komponente/topbar.html",
}

// saSidebar su šabloni koji koriste base layout (sidebar + topbar)
var saSidebar = []string{
	"admin_korisnici", "admin_profil", "admin_login_istorija",
	"dashboard",
	"dobavljaci", "dobavljac_forma",
	"izvestaji",
	"kategorije",
	"klijenti", "klijent_forma",
	"magacin", "magacin_forma",
	"nabavke", "nabavka_forma", "nabavka_detalji",
	"podesavanja", "podesavanja_opste", "podesavanja_izgled", "podesavanja_sistem",
	"podsetnici", "podsetnik_forma",
	"prodaja", "prodaja_detalji", "prodaja_forma",
	"servis", "servis_forma", "servis_detalji",
}

// standalone su šabloni bez base layouta
var standaloneIme = []string{
	"prijava", "setup", "totp_provera", "prodaja_stampa",
}

// KreirajKes parsuje sve šablone iz fsys i vraća ih keširane u mapi
func KreirajKes(fsys fs.FS) (map[string]*template.Template, error) {
	kes := make(map[string]*template.Template)

	for _, ime := range saSidebar {
		fajlovi := make([]string, len(bazniSabloni), len(bazniSabloni)+1)
		copy(fajlovi, bazniSabloni)
		fajlovi = append(fajlovi, "web/templates/stranice/"+ime+".html")
		t, err := template.ParseFS(fsys, fajlovi...)
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
	var tmpl *template.Template

	if h.Templates != nil {
		t, ok := h.Templates[ime]
		if !ok {
			log.Printf("kes: šablon '%s' nije pronađen", ime)
			http.Error(w, "Greška pri učitavanju stranice", http.StatusInternalServerError)
			return
		}
		tmpl = t
	} else {
		fajlovi := make([]string, len(bazniSabloni), len(bazniSabloni)+1)
		copy(fajlovi, bazniSabloni)
		fajlovi = append(fajlovi, "web/templates/stranice/"+ime+".html")
		var err error
		if tmpl, err = template.ParseFS(h.TemplatesFS, fajlovi...); err != nil {
			log.Printf("greška pri parsiranju šablona %s: %v", ime, err)
			http.Error(w, "Greška pri učitavanju stranice", http.StatusInternalServerError)
			return
		}
	}

	if err := tmpl.ExecuteTemplate(w, "base", podaci); err != nil {
		log.Printf("greška pri renderovanju šablona %s: %v", ime, err)
		http.Error(w, "Greška pri prikazu stranice", http.StatusInternalServerError)
	}
}

// renderujStandalone renderuje šablon bez base layouta (prijava, setup, itd.)
func (h *Handler) renderujStandalone(w http.ResponseWriter, ime string, podaci any) {
	var tmpl *template.Template

	if h.Templates != nil {
		t, ok := h.Templates[ime]
		if !ok {
			log.Printf("kes: standalone šablon '%s' nije pronađen", ime)
			http.Error(w, "Greška pri učitavanju stranice", http.StatusInternalServerError)
			return
		}
		tmpl = t
	} else {
		var err error
		if tmpl, err = template.ParseFS(h.TemplatesFS, "web/templates/stranice/"+ime+".html"); err != nil {
			log.Printf("greška pri parsiranju šablona %s: %v", ime, err)
			http.Error(w, "Greška pri učitavanju stranice", http.StatusInternalServerError)
			return
		}
	}

	if err := tmpl.Execute(w, podaci); err != nil {
		log.Printf("greška pri renderovanju šablona %s: %v", ime, err)
		http.Error(w, "Greška pri prikazu stranice", http.StatusInternalServerError)
	}
}
