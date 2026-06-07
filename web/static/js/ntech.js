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
            this.blur = parseInt(this.$el.dataset.blur) || 12
            this.opacity = parseInt(this.$el.dataset.opacity) || 50
            this.blurPozadine = parseInt(this.$el.dataset.blurPozadine) || 0
            this.glassOpacity = parseInt(this.$el.dataset.glassOpacity) || 10
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
        stavke: [{artikal_id: '', kolicina: 1, cena: 0}],
        artikliOpcije: [],
        isMobile: false,
        init() {
            this.artikliOpcije = window._ntechArtikli || []
            this.isMobile = window.matchMedia('(max-width: 768px)').matches
            window.matchMedia('(max-width: 768px)').addEventListener('change', e => {
                this.isMobile = e.matches
            })
        },
        dodajStavku() {
            this.stavke.push({artikal_id: '', kolicina: 1, cena: 0})
        },
        ukloniStavku(i) {
            if (this.stavke.length > 1) this.stavke.splice(i, 1)
        },
        popuniCenu(stavka) {
            const a = this.artikliOpcije.find(x => x.id == stavka.artikal_id)
            if (a) stavka.cena = a.cena
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
            return (parseFloat(s.kolicina) * parseFloat(s.cena) || 0).toFixed(2)
        },
        ukupnoSvega() {
            return this.stavke.reduce((z, s) => z + (parseFloat(s.kolicina) * parseFloat(s.cena) || 0), 0).toFixed(2)
        }
    }))

    // forma za nabavku
    Alpine.data('nabavkaForma', () => ({
        stavke: [{artikal_id: '', kolicina: 1, cena: 0}],
        artikliOpcije: [],
        isMobile: false,
        modal: false,
        modalUcitavanje: false,
        modalGreska: '',
        modalNaziv: '',
        modalKategorijaID: '',
        modalCena: '',
        init() {
            this.artikliOpcije = window._ntechArtikli || []
            this.isMobile = window.matchMedia('(max-width: 768px)').matches
            window.matchMedia('(max-width: 768px)').addEventListener('change', e => {
                this.isMobile = e.matches
            })
        },
        dodajStavku() {
            this.stavke.push({artikal_id: '', kolicina: 1, cena: 0})
        },
        ukloniStavku(i) {
            if (this.stavke.length > 1) this.stavke.splice(i, 1)
        },
        ukupnoStavke(s) {
            return (parseFloat(s.kolicina) * parseFloat(s.cena) || 0).toFixed(2)
        },
        ukupnoSvega() {
            return this.stavke.reduce((z, s) => z + (parseFloat(s.kolicina) * parseFloat(s.cena) || 0), 0).toFixed(2)
        },
        otvoriModal() {
            this.modal = true
            this.modalGreska = ''
            this.modalNaziv = ''
            this.modalKategorijaID = ''
            this.modalCena = ''
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
            if (this.modalCena) params.append('prodajna_cena', this.modalCena)
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
                this.artikliOpcije.push({id: noviArtikal.id, naziv: noviArtikal.naziv})
                this.zatvoriModal()
            } catch {
                this.modalGreska = 'Greška pri komunikaciji sa serverom.'
            } finally {
                this.modalUcitavanje = false
            }
        }
    }))

})
