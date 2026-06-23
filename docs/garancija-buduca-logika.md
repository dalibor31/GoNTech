# Garancija — buduća logika (arhiva koda)

## Plan (zašto)

Trenutno se „Garancija do" unosi kao konkretan datum. To nema smisla dok je uređaj
tek primljen (status **Primljeno**) — popravka još nije ni počela, pa ni datum
garancije nije poznat.

**Buduća logika:**
- Garancija se izražava kao **broj dana od datuma završetka** (podesivo, kao i sada
  preko `servis_garancija_meseci` / mogući novi `servis_garancija_dana`).
- Dok je nalog u radu (Primljeno, U popravci…), klijentu se prikazuje samo
  **podrazumevana garancija** kao informacija (npr. „garancija: 12 meseci od završetka").
- **Pravi datum garancije** (`garancija_do`) upisuje se tek kad nalog pređe u
  **Završeno** → `datum_zavrsetka + garantni period`.

Zbog toga je „Garancija do" uklonjena iz forme **Izmeni nalog** (vidi commit), a
ovde je arhiviran kod da se kasnije ponovo upotrebi / prilagodi.

## Arhivirani kod — „Garancija do" blok iz `servis_forma.html` (Izmeni nalog)

```html
{{if .Izmena}}
<!-- garancija — isti izgled kao popup „Garancija do" u detaljima naloga;
     „Bez garancije" isključuje datum, „Podrazumevano" vraća prijem + meseci -->
<div>
    <label class="polje-labela">Garancija do</label>
    <div style="display:flex;align-items:center;gap:10px;">
        <input type="date" name="garancija_do"
            value="{{if .Nalog.GarancijaDo}}{{.Nalog.GarancijaDo.Format "2006-01-02"}}{{else}}{{.GarancijaDefault}}{{end}}"
            min="{{.Nalog.DatumPrijema.Format "2006-01-02"}}"
            {{if .BezGarancije}}disabled{{end}}
            style="flex:1;min-width:0;{{if .BezGarancije}}opacity:.5;{{end}}">
        <label class="tip-opcija{{if .BezGarancije}} tip-opcija-akt{{end}}" style="white-space:nowrap;">
            <input type="checkbox" name="bez_garancije" value="1"{{if .BezGarancije}} checked{{end}} style="margin:0;width:auto;"
                onchange="var d=this.closest('div').querySelector('[name=garancija_do]'); d.disabled=this.checked; d.style.opacity=this.checked?'.5':''; this.closest('label').classList.toggle('tip-opcija-akt',this.checked);">
            Bez garancije
        </label>
    </div>
    <button type="button" class="btn-sekundarno" style="margin-top:8px;"
        onclick="var f=this.closest('form'); var d=f.querySelector('[name=garancija_do]'); d.disabled=false; d.style.opacity=''; d.value='{{.GarancijaDefault}}'; var c=f.querySelector('[name=bez_garancije]'); c.checked=false; c.closest('label').classList.remove('tip-opcija-akt'); d.dispatchEvent(new Event('change',{bubbles:true}));">Podrazumevano</button>
</div>
{{end}}
```

## Backend koji OSTAJE (i dalje koristi modal „Garancija do" u detaljima)

Nije uklonjeno — ovo su delovi koje treba znati pri ponovnoj upotrebi:

- `internal/handler/servis.go`:
  - `defaultGarancija(datumPrijema, podesavanja)` — prijem + `servis_garancija_meseci`.
    Za buduću logiku zameniti osnovicu `datum_zavrsetka` umesto `datum_prijema`.
  - `garancijaPrePrijema(garancija, prijem)` — validacija da garancija nije pre prijema.
  - `AzurirajGaranciju` (ruta `POST /servis/{id}/garancija`) — snima `garancija_do`,
    prazno / „bez garancije" → NULL.
  - U `parseFormuNaloga`: blok koji čita `garancija_do` + `bez_garancije`.
  - `BezGarancije` flag (čita NULL stanje iz baze) i `GarancijaDefault` (prijem + meseci)
    u `PodaciDetaljiNaloga` / `PodaciFormeNaloga`.
- `internal/db/sqlite/servis.go`: `AzurirajGaranciju` repo metoda; kolona `garancija_do`.
- Podešavanje: `servis_garancija_meseci` (Podešavanja → Servis).

## TODO kad se vraća

1. Dodati prikaz „podrazumevana garancija" (informativno) na nalogu u statusu Primljeno.
2. Pri prelasku u **Završeno** automatski izračunati `garancija_do = datum_zavrsetka + period`.
3. Razmotriti period u danima umesto/pored meseci.
