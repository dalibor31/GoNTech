// otvara/zatvara podmeni u sidebaru — radi i kad je sidebar skupljen i kad je proširen
// (sidebar ostaje u zatečenom stanju). U isto vreme sme biti otvoren samo jedan podmeni.
function ntechTogglePodmeni(btn) {
    var podmeni = btn.nextElementSibling;
    if (!podmeni) return;
    var jeOtvoren = podmeni.classList.contains('otvoren');

    // zatvori sve podmenije i vrati njihove strelice — međusobna isključivost
    document.querySelectorAll('#sidebar .nav-podmeni').forEach(function(el) {
        el.classList.remove('otvoren');
        var dugme = el.previousElementSibling;
        if (dugme) {
            var s = dugme.querySelector('.nav-strelica svg');
            if (s) s.style.transform = 'rotate(0deg)';
        }
    });

    // ako kliknuti nije već bio otvoren — otvori ga
    if (!jeOtvoren) {
        podmeni.classList.add('otvoren');
        var svg = btn.querySelector('.nav-strelica svg');
        if (svg) svg.style.transform = 'rotate(180deg)';
    }
}

// registruje klik listenere na podmeni dugmad — sidebar se nikad ne menja, poziva se jednom
function ntechDodajPodmeniListenere() {
    document.querySelectorAll('#sidebar [data-podmeni-dugme]').forEach(function(btn) {
        btn.addEventListener('click', function(e) {
            e.stopPropagation();
            ntechTogglePodmeni(btn);
        });
    });
}

// sprečava CSS tranziciju na podmeniima koji su već otvoreni pri inicijalnom učitavanju
// (bez ovoga, max-height animira od 0 do 300px pri svakom swap-u)
function ntechInicijalizujPodmeni() {
    document.querySelectorAll('.nav-podmeni.otvoren').forEach(function(el) {
        el.classList.add('bez-tranzicije');
        requestAnimationFrame(function() {
            requestAnimationFrame(function() {
                el.classList.remove('bez-tranzicije');
            });
        });
    });
}

document.addEventListener('alpine:init', () => {

    // sidebar — podmeni "Moj profil"
    Alpine.data('sidebarProfil', () => ({
        otvoren: false,
        init() {
            this.otvoren = this.$el.dataset.otvoren === 'true'
        }
    }))

    // sidebar — podmeni "Podešavanja"
    Alpine.data('sidebarPodesavanja', () => ({
        otvoren: false,
        init() {
            this.otvoren = this.$el.dataset.otvoren === 'true'
        }
    }))

    // profil — odabir teme
    Alpine.data('profilTemaOdabir', () => ({
        tema: 'tamna',
        init() {
            this.tema = this.$el.dataset.tema || 'tamna'
        }
    }))

    // preview login pozadine (podesavanja.html, podesavanja_izgled.html)
    Alpine.data('loginPozadinaPreview', () => ({
        pozadina: '',
        blurPozadine: 0,
        blurKartice: 12,
        opacity: 50,
        zatamnjenjeKartice: 0,
        init() {
            this.pozadina = this.$el.dataset.pozadina || ''
            this.blurPozadine = parseInt(this.$el.dataset.blurPozadine) || 0
            this.blurKartice = parseInt(this.$el.dataset.blurKartice) || 12
            this.opacity = parseInt(this.$el.dataset.opacity) || 50
            this.zatamnjenjeKartice = parseInt(this.$el.dataset.zatamnjenjeKartice) || 0
        },
        stilPozadine() {
            const bgCss = this.pozadina ? "background:url('" + this.pozadina + "') center/cover;" : 'background:#1a2033;'
            const inset = this.blurPozadine > 0 ? '-20px' : '0'
            return 'position:absolute;inset:' + inset + ';' + bgCss + 'filter:blur(' + this.blurPozadine + 'px);z-index:0;'
        },
        stilOverlay() {
            return 'position:absolute;inset:0;z-index:1;pointer-events:none;background:rgba(0,0,0,' + (this.opacity / 100) + ')'
        },
        stilKartice() {
            return 'width:140px;border-radius:10px;background:rgba(0,0,0,' + (this.zatamnjenjeKartice / 100) + ');backdrop-filter:blur(' + this.blurKartice + 'px);-webkit-backdrop-filter:blur(' + this.blurKartice + 'px);border:1px solid rgba(255,255,255,0.18);box-shadow:0 8px 32px rgba(0,0,0,0.3);padding:14px 16px;'
        }
    }))

    // preview lične pozadine (profil_tema.html)
    Alpine.data('lokalnaPozadinaPreview', () => ({
        pozadina: '',
        blur: 12,
        opacity: 50,
        blurPozadine: 0,
        glassOpacity: 10,
        init() {
            this.pozadina = this.$el.dataset.pozadina || ''
            // ne koristimo „|| podrazumevano" jer je 0 validna vrednost a falsy — pala bi na podrazumevano
            this.blur = this.broj(this.$el.dataset.blur, 12)
            this.opacity = this.broj(this.$el.dataset.opacity, 50)
            this.blurPozadine = this.broj(this.$el.dataset.blurPozadine, 0)
            this.glassOpacity = this.broj(this.$el.dataset.glassOpacity, 10)
        },
        // vraća ceo broj iz vrednosti; ako nije broj, vraća podrazumevano (0 ostaje 0)
        broj(vrednost, podrazumevano) {
            const n = parseInt(vrednost, 10)
            return Number.isNaN(n) ? podrazumevano : n
        },
        stilPozadine() {
            const bgCss = this.pozadina ? "background:url('" + this.pozadina + "') center/cover;" : 'background:#1a2033;'
            return 'position:absolute;inset:0;' + bgCss + 'z-index:0;filter:blur(' + this.blurPozadine + 'px);-webkit-filter:blur(' + this.blurPozadine + 'px);transform:scale(1.05);'
        },
        stilOverlay() {
            return 'position:absolute;inset:0;z-index:1;pointer-events:none;background:rgba(0,0,0,' + (this.opacity / 100) + ')'
        },
        stilSidebar() {
            return 'position:absolute;top:0;left:0;bottom:0;z-index:2;width:56px;background:rgba(0,0,0,' + (this.glassOpacity / 100) + ');backdrop-filter:blur(' + this.blur + 'px);-webkit-backdrop-filter:blur(' + this.blur + 'px);border-right:1px solid rgba(255,255,255,0.15);display:flex;flex-direction:column;align-items:center;padding-top:10px;gap:12px'
        },
        stilKartica() {
            return 'height:36px;border-radius:6px;background:rgba(0,0,0,' + (this.glassOpacity / 100) + ');backdrop-filter:blur(' + this.blur + 'px);-webkit-backdrop-filter:blur(' + this.blur + 'px);border:1px solid rgba(255,255,255,0.15);display:flex;flex-direction:column;justify-content:center;padding:0 10px'
        }
    }))

    // forma za prodaju
    Alpine.data('prodajaForma', () => ({
        stavke: [{artikal_id: '', kolicina: 1, cena: 0, cena_sa_pdv: 0, pdv_stopa: 20, popust: 0, kategorija_naziv: ''}],
        artikliOpcije: [],
        pretragaArtikal: '',
        pdvObveznik: true,
        isMobile: false,
        init() {
            this.artikliOpcije = window._ntechArtikli || []
            this.pdvObveznik = window._ntechPdvObveznik === true
            this.isMobile = window.matchMedia('(max-width: 768px)').matches
            window.matchMedia('(max-width: 768px)').addEventListener('change', e => {
                this.isMobile = e.matches
            })
        },
        get filtriraniArtikli() {
            const q = this.pretragaArtikal.trim().toLowerCase()
            if (!q) return this.artikliOpcije.slice(0, 50)
            return this.artikliOpcije.filter(a =>
                (a.naziv || '').toLowerCase().includes(q) ||
                (a.sifra || '').toLowerCase().includes(q) ||
                (a.barkod || '').toLowerCase().includes(q)
            ).slice(0, 30)
        },
        dodajStavku() {
            this.stavke.push({artikal_id: '', kolicina: 1, cena: 0, cena_sa_pdv: 0, pdv_stopa: 20, popust: 0, kategorija_naziv: ''})
        },
        ukloniStavku(i) {
            if (this.stavke.length > 1) this.stavke.splice(i, 1)
        },
        popuniCenu(stavka) {
            const a = this.artikliOpcije.find(x => x.id == stavka.artikal_id)
            if (a) {
                stavka.cena = a.cena || 0
                stavka.cena_sa_pdv = a.cena_sa_pdv || 0
                stavka.pdv_stopa = a.pdv_stopa !== undefined ? a.pdv_stopa : 20
                stavka.kategorija_naziv = a.kategorija_naziv || ''
            }
        },
        prikaziCenu(stavka) {
            return this.pdvObveznik ? (stavka.cena || 0) : (stavka.cena_sa_pdv || stavka.cena || 0)
        },
        dostupnaKolicina(i) {
            const stavka = this.stavke[i]
            if (!stavka.artikal_id) return null
            const a = this.artikliOpcije.find(x => x.id == stavka.artikal_id)
            if (!a) return null
            const ostale = this.stavke.reduce((sum, s, j) =>
                sum + (j !== i && s.artikal_id == stavka.artikal_id ? (parseInt(s.kolicina) || 0) : 0), 0)
            return a.kolicina - ostale
        },
        prekoracenje(i) {
            const d = this.dostupnaKolicina(i)
            if (d === null) return false
            return (parseInt(this.stavke[i].kolicina) || 0) > d
        },
        imaPrekoracenja() {
            return this.stavke.some((_, i) => this.prekoracenje(i))
        },
        ukupnoStavke(s) {
            const cena = parseFloat(s.cena) || 0
            const popust = parseFloat(s.popust) || 0
            const cenaPosle = popust > 0 ? cena * (1 - popust/100) : cena
            return (parseFloat(s.kolicina) * cenaPosle || 0).toFixed(2)
        },
        ukupnoSvega() {
            return this.stavke.reduce((z, s) => {
                const cena = parseFloat(s.cena) || 0
                const popust = parseFloat(s.popust) || 0
                const cenaPosle = popust > 0 ? cena * (1 - popust/100) : cena
                return z + (parseFloat(s.kolicina) * cenaPosle || 0)
            }, 0).toFixed(2)
        },
        // fmtDin formatira broj sa separatorom hiljada (tačka) i 2 decimale (zarez): 1234567.5 → "1.234.567,50"
        fmtDin(v) {
            const n = parseFloat(v) || 0
            const [ceo, dec] = Math.abs(n).toFixed(2).split('.')
            const saTackama = ceo.replace(/\B(?=(\d{3})+(?!\d))/g, '.')
            return (n < 0 ? '-' : '') + saTackama + ',' + dec
        }
    }))

    // forma servisnog naloga — izbor postojećeg ili unos novog klijenta.
    // Pisano CSP-safe (bez složenih izraza u atributima): uslovi idu kroz metode/gettere.
    Alpine.data('klijentNalog', () => ({
        klijenti: [],
        tip: 'fizicko',
        pretraga: '',
        izabrani: null,
        izabraniId: 0,
        prikaziListu: false,
        // polja novog klijenta (vezana x-model-om na inpute)
        noviIme: '',
        noviPrezime: '',
        noviNaziv: '',
        init() {
            this.klijenti = window.__klijentiNaloga || []
            // kod izmene naloga pretpopuni već vezanog klijenta
            const id = window.__izabraniKlijentId || 0
            if (id) {
                const k = this.klijenti.find(x => x.ID === id)
                if (k) {
                    this.izabrani = k
                    this.izabraniId = k.ID
                    this.tip = k.Tip || 'fizicko'
                }
            }
        },
        get nijeIzabran() {
            return !this.izabrani
        },
        get vrednostKlijentId() {
            return this.izabraniId || ''
        },
        // filtrira klijente po tipu i početnim karakterima unosa (najviše 8)
        get filtrirani() {
            const q = this.pretraga.trim().toLowerCase()
            const tip = this.tip
            return this.klijenti.filter(k => {
                if ((k.Tip || 'fizicko') !== tip) return false
                if (q === '') return true
                // za fizičko proveravamo i puno ime ("Ivan Lazić") da bi radio unos
                // tipa "Ivan L", a i samo ime ili samo prezime zasebno
                const punoIme = ((k.Ime || '') + ' ' + (k.Prezime || '')).trim()
                const polja = (tip === 'pravno') ? [k.NazivFirme] : [punoIme, k.Ime, k.Prezime]
                return polja.some(p => (p || '').toLowerCase().startsWith(q))
            }).slice(0, 100)
        },
        jeTip(t) {
            return this.tip === t
        },
        klasaTip(t) {
            return this.tip === t ? 'tip-opcija-akt' : ''
        },
        naziv(k) {
            if (!k) return ''
            return (k.Tip === 'pravno')
                ? (k.NazivFirme || '')
                : ((k.Ime || '') + ' ' + (k.Prezime || '')).trim()
        },
        detalj(k) {
            if (!k) return ''
            const d = []
            if (k.Mesto) d.push(k.Mesto)
            if (k.Telefon) d.push(k.Telefon)
            return d.join(' · ')
        },
        izaberi(k) {
            this.izabrani = k
            this.izabraniId = k.ID
            this.pretraga = ''
            this.prikaziListu = false
        },
        ocisti() {
            this.izabrani = null
            this.izabraniId = 0
        },
        // Enter u pretrazi kad ništa nije izabrano → priprema unos novog klijenta:
        // prvu reč upisuje kao ime, ostatak kao prezime (za firmu ceo tekst kao naziv)
        potvrdiNovog() {
            const q = this.pretraga.trim()
            this.prikaziListu = false
            if (!q) return
            if (this.tip === 'pravno') {
                this.noviNaziv = q
            } else {
                const delovi = q.split(/\s+/)
                this.noviIme = delovi[0] || ''
                this.noviPrezime = delovi.slice(1).join(' ')
            }
        },
        promeniTip(t) {
            this.tip = t
            this.ocisti()
            this.pretraga = ''
            this.noviIme = ''
            this.noviPrezime = ''
            this.noviNaziv = ''
        }
    }))

    // forma za nabavku
    Alpine.data('nabavkaForma', () => ({
        stavke: [{artikal_id: '', kolicina: 1, cena: 0, marza: 0, prodajna: 0}],
        artikliOpcije: [],
        dobavljacId: '',         // izabrani dobavljač nabavke — filtrira listu artikala
        brojRacuna: '',          // broj računa dobavljača — obavezan za PDV KPR
        prikaziSveArtikle: false, // true = prikaži sve artikle, ne samo dobavljačeve
        marzaDefault: 0,
        troskovi: [],            // zavisni troškovi {naziv, iznos}
        metodRaspodele: 'vrednost', // 'vrednost' ili 'kolicina'
        pdvObveznik: true,       // da li firma obračunava PDV (utiče na prodajnu cenu)
        isMobile: false,
        modal: false,
        modalUcitavanje: false,
        modalGreska: '',
        modalNaziv: '',
        modalKategorijaID: '',
        modalOpis: '',
        modalKolicina: '',
        modalKolicinaMin: '',
        modalNabavnaCena: '',
        modalCena: '',
        modalLokacija: '',
        modalNapomena: '',
        modalDob: false,
        modalDobUcitavanje: false,
        modalDobGreska: '',
        modalDobNaziv: '',
        modalDobKontakt: '',
        modalDobTelefon: '',
        modalDobEmail: '',
        modalDobPib: '',
        modalDobMesto: '',
        modalDobNapomena: '',
        init() {
            this.artikliOpcije = window._ntechArtikli || []
            this.marzaDefault = parseFloat(window._ntechMarza) || 0
            this.pdvObveznik = window._ntechPdvObveznik === true
            this.stavke.forEach(s => { s.marza = this.marzaDefault })
            this.isMobile = window.matchMedia('(max-width: 768px)').matches
            window.matchMedia('(max-width: 768px)').addEventListener('change', e => {
                this.isMobile = e.matches
            })
            const resetujNedostupne = () => {
                if (this.prikaziSveArtikle) return
                const dostupni = new Set(this.artikliZaDobavljaca().map(a => String(a.id)))
                this.stavke.forEach(s => {
                    if (s.artikal_id && !dostupni.has(String(s.artikal_id))) {
                        s.artikal_id = ''
                        s.cena = 0
                        s.marza = this.marzaDefault
                        s.prodajna = 0
                    }
                })
                this.preracunajSve()
            }
            this.$watch('dobavljacId', resetujNedostupne)
            this.$watch('prikaziSveArtikle', resetujNedostupne)
        },
        dodajStavku() {
            // ne dozvoli novu stavku dok poslednja nema izabran artikal
            const poslednja = this.stavke[this.stavke.length - 1]
            if (poslednja && !poslednja.artikal_id) {
                if (window.ntechToast) window.ntechToast('Prvo izaberi artikal u poslednjoj stavci.', 'greska')
                return
            }
            this.stavke.push({artikal_id: '', kolicina: 1, cena: 0, marza: this.marzaDefault, prodajna: 0})
            this.preracunajSve()
        },
        // artikli za prikaz u stavkama: bez dobavljača (ili uz "prikaži sve") svi, inače samo njegovi
        artikliZaDobavljaca() {
            if (!this.dobavljacId || this.prikaziSveArtikle) return this.artikliOpcije
            const did = Number(this.dobavljacId)
            return this.artikliOpcije.filter(a => Array.isArray(a.dobavljaci) && a.dobavljaci.includes(did))
        },
        // PDV stopa izabranog artikla (iz JSON liste) — za obračun prodajne cene.
        // Ako firma nije PDV obveznik, PDV se ne dodaje na prodajnu cenu (stopa = 0).
        pdvStopa(artikalId) {
            if (!this.pdvObveznik) return 0
            const a = this.artikliOpcije.find(x => String(x.id) === String(artikalId))
            return a ? (parseFloat(a.pdv_stopa) || 0) : 0
        },
        // pri izboru artikla predloži maržu: artikal → kategorija → globalna, pa izračunaj prodajnu
        izaberiArtikal(s) {
            const a = this.artikliOpcije.find(x => String(x.id) === String(s.artikal_id))
            if (a) {
                if (a.nabavna_cena != null) s.cena = a.nabavna_cena
                if (a.marza != null) s.marza = a.marza
                else if (a.kategorija_marza != null) s.marza = a.kategorija_marza
                else s.marza = this.marzaDefault
            } else {
                s.cena = 0
                s.marza = this.marzaDefault
                s.prodajna = 0
            }
            this.izracunajProdajnu(s)
            this.preracunajSve()
        },
        // ukupan zavisni trošak nabavke
        ukupanTrosak() {
            return this.troskovi.reduce((z, t) => z + (parseFloat(t.iznos) || 0), 0)
        },
        // osnovica raspodele po izabranom metodu (zbir po svim stavkama)
        osnovicaRaspodele() {
            return this.stavke.reduce((z, s) => {
                const kol = parseFloat(s.kolicina) || 0
                return z + (this.metodRaspodele === 'kolicina' ? kol : kol * (parseFloat(s.cena) || 0))
            }, 0)
        },
        // kalkulativna nabavna cena po komadu (fakturna + raspodeljeni trošak) — isto kao server
        kalkNabavna(s) {
            const cena = parseFloat(s.cena) || 0
            const kol = parseFloat(s.kolicina) || 0
            const trosak = this.ukupanTrosak()
            const osn = this.osnovicaRaspodele()
            if (trosak <= 0 || osn <= 0 || kol <= 0) return cena
            const baza = this.metodRaspodele === 'kolicina' ? kol : kol * cena
            const trosakPoKomadu = (trosak * (baza / osn)) / kol
            return Math.round((cena + trosakPoKomadu) * 100) / 100
        },
        // prodajna (sa PDV) = kalkulativna nabavna × (1 + marža/100) × (1 + pdvStopa/100)
        izracunajProdajnu(s) {
            const nabavna = this.kalkNabavna(s)
            const marza = parseFloat(s.marza) || 0
            const pdv = this.pdvStopa(s.artikal_id)
            s.prodajna = Math.round(nabavna * (1 + marza / 100) * (1 + pdv / 100) * 100) / 100
        },
        // obrnuti smer: iz ručno unete prodajne cene izvedi maržu (%)
        // marža = (prodajna / (nabavna × (1 + pdv/100)) − 1) × 100
        izracunajMarzu(s) {
            const nabavna = this.kalkNabavna(s)
            const pdv = this.pdvStopa(s.artikal_id)
            const osnovica = nabavna * (1 + pdv / 100)
            if (osnovica <= 0) return // bez nabavne cene marža se ne može izvesti
            const prodajna = parseFloat(s.prodajna) || 0
            s.marza = Math.round(((prodajna / osnovica) - 1) * 100 * 100) / 100
        },
        // raspodela zavisi od svih stavki — promena troška/metoda/količine/cene preračunava sve
        preracunajSve() {
            this.stavke.forEach(s => this.izracunajProdajnu(s))
        },
        dodajTrosak() {
            const poslednji = this.troskovi[this.troskovi.length - 1]
            if (poslednji && !poslednji.naziv.trim()) {
                if (window.ntechToast) window.ntechToast('Prvo unesi naziv poslednjeg troška.', 'greska')
                return
            }
            this.troskovi.push({naziv: '', iznos: 0})
        },
        ukloniTrosak(i) {
            this.troskovi.splice(i, 1)
            this.preracunajSve()
        },
        ukloniStavku(i) {
            if (this.stavke.length > 1) {
                this.stavke.splice(i, 1)
            } else {
                // poslednja stavka — ne brišemo red nego ga resetujemo na prazno
                this.stavke[0] = {artikal_id: '', kolicina: 1, cena: 0, marza: this.marzaDefault, prodajna: 0}
            }
            this.preracunajSve()
        },
        ukupnoStavke(s) {
            return (parseFloat(s.kolicina) * parseFloat(s.cena) || 0).toFixed(2)
        },
        ukupnoSvega() {
            return this.stavke.reduce((z, s) => z + (parseFloat(s.kolicina) * parseFloat(s.cena) || 0), 0).toFixed(2)
        },
        // fmtDin formatira broj sa separatorom hiljada (tačka) i 2 decimale (zarez): 1234567.5 → "1.234.567,50"
        fmtDin(v) {
            const n = parseFloat(v) || 0
            const [ceo, dec] = Math.abs(n).toFixed(2).split('.')
            const saTackama = ceo.replace(/\B(?=(\d{3})+(?!\d))/g, '.')
            return (n < 0 ? '-' : '') + saTackama + ',' + dec
        },
        otvoriModal() {
            this.modal = true
            this.modalGreska = ''
            this.modalNaziv = ''
            this.modalKategorijaID = ''
            this.modalOpis = ''
            this.modalKolicina = ''
            this.modalKolicinaMin = ''
            this.modalNabavnaCena = ''
            this.modalCena = ''
            this.modalLokacija = ''
            this.modalNapomena = ''
            this.$nextTick(() => this.$refs.modalNazivInput && this.$refs.modalNazivInput.focus())
        },
        zatvoriModal() {
            this.modal = false
        },
        async sacuvajArtikal() {
            if (!this.modalNaziv.trim()) {
                this.modalGreska = 'Naziv artikla je obavezan.'
                return
            }
            this.modalUcitavanje = true
            this.modalGreska = ''
            const params = new URLSearchParams()
            params.append('naziv', this.modalNaziv.trim())
            if (this.modalKategorijaID) params.append('kategorija_id', this.modalKategorijaID)
            params.append('opis', this.modalOpis.trim())
            if (this.modalKolicina) params.append('kolicina', this.modalKolicina)
            if (this.modalKolicinaMin) params.append('kolicina_min', this.modalKolicinaMin)
            if (this.modalNabavnaCena) params.append('nabavna_cena', this.modalNabavnaCena)
            if (this.modalCena) params.append('prodajna_cena', this.modalCena)
            params.append('lokacija', this.modalLokacija.trim())
            params.append('napomena', this.modalNapomena.trim())
            params.append('_csrf', document.querySelector('meta[name=csrf-token]')?.content || '')
            try {
                const odgovor = await fetch('/magacin/novi', {
                    method: 'POST',
                    headers: {'X-Requested-With': 'fetch', 'Content-Type': 'application/x-www-form-urlencoded'},
                    body: params
                })
                if (!odgovor.ok) {
                    this.modalGreska = 'Greška pri čuvanju artikla. Pokušajte ponovo.'
                    return
                }
                const noviArtikal = await odgovor.json()
                // veži novi artikal za izabranog dobavljača da se odmah prikaže u filtriranoj listi
                // (veza se trajno upisuje u bazu pri čuvanju nabavke — auto-veza)
                const dob = this.dobavljacId ? [Number(this.dobavljacId)] : []
                this.artikliOpcije.push({id: noviArtikal.id, naziv: noviArtikal.naziv, dobavljaci: dob})
                this.zatvoriModal()
            } catch {
                this.modalGreska = 'Greška pri komunikaciji sa serverom.'
            } finally {
                this.modalUcitavanje = false
            }
        },
        otvoriModalDobavljac() {
            this.modalDob = true
            this.modalDobGreska = ''
            this.modalDobNaziv = ''
            this.modalDobKontakt = ''
            this.modalDobTelefon = ''
            this.modalDobEmail = ''
            this.modalDobPib = ''
            this.modalDobMesto = ''
            this.modalDobNapomena = ''
            this.$nextTick(() => this.$refs.modalDobNazivInput && this.$refs.modalDobNazivInput.focus())
        },
        zatvoriModalDobavljac() {
            this.modalDob = false
        },
        async sacuvajDobavljaca() {
            if (!this.modalDobNaziv.trim()) {
                this.modalDobGreska = 'Naziv dobavljača je obavezan.'
                return
            }
            this.modalDobUcitavanje = true
            this.modalDobGreska = ''
            const params = new URLSearchParams()
            params.append('naziv', this.modalDobNaziv.trim())
            params.append('kontakt_osoba', this.modalDobKontakt.trim())
            params.append('telefon', this.modalDobTelefon.trim())
            params.append('email', this.modalDobEmail.trim())
            params.append('pib', this.modalDobPib.trim())
            params.append('mesto', this.modalDobMesto.trim())
            params.append('napomena', this.modalDobNapomena.trim())
            params.append('_csrf', document.querySelector('meta[name=csrf-token]')?.content || '')
            try {
                const odgovor = await fetch('/dobavljaci/novi', {
                    method: 'POST',
                    headers: {'X-Requested-With': 'fetch', 'Content-Type': 'application/x-www-form-urlencoded'},
                    body: params
                })
                if (!odgovor.ok) {
                    const greska = await odgovor.json().catch(() => null)
                    this.modalDobGreska = (greska && greska.greska) || 'Greška pri čuvanju dobavljača. Pokušajte ponovo.'
                    return
                }
                const novi = await odgovor.json()
                // dodaj novog dobavljača u padajuću listu i odmah ga izaberi
                const opcija = document.createElement('option')
                opcija.value = novi.id
                opcija.textContent = novi.naziv
                this.$refs.selDobavljac.appendChild(opcija)
                this.$refs.selDobavljac.value = novi.id
                this.dobavljacId = String(novi.id)
                this.zatvoriModalDobavljac()
            } catch {
                this.modalDobGreska = 'Greška pri komunikaciji sa serverom.'
            } finally {
                this.modalDobUcitavanje = false
            }
        }
    }))

})

// za prvo učitavanje (defer script) — sprečava animaciju podmenia koji su inicijalno otvoreni
ntechInicijalizujPodmeni()
ntechDodajPodmeniListenere()

// klizač (input[type=range].klizac): popunjava plavom deo pre glave u Chromium-u
// preko CSS promenljive --popunjeno. Firefox to radi sam (::-moz-range-progress).
function ntechAzurirajKlizac(el) {
    var min = parseFloat(el.min) || 0
    var max = parseFloat(el.max)
    if (isNaN(max)) max = 100
    var v = parseFloat(el.value) || 0
    var procenat = max > min ? ((v - min) / (max - min)) * 100 : 0
    el.style.setProperty('--popunjeno', procenat + '%')
}
function ntechInicijalizujKlizace() {
    document.querySelectorAll('input[type="range"].klizac').forEach(ntechAzurirajKlizac)
}
(function() {
    if (window._ntechKlizacDodato) return
    window._ntechKlizacDodato = true
    // delegirani listener — hvata i klizače ubačene kasnije (HTMX swap)
    document.addEventListener('input', function(e) {
        var t = e.target
        if (t && t.classList && t.classList.contains('klizac')) ntechAzurirajKlizac(t)
    })
    // početno popunjavanje: posle učitavanja, posle HTMX swap-a i kad Alpine postavi vrednosti
    document.addEventListener('DOMContentLoaded', ntechInicijalizujKlizace)
    document.addEventListener('htmx:afterSettle', ntechInicijalizujKlizace)
    document.addEventListener('alpine:initialized', ntechInicijalizujKlizace)
})()
