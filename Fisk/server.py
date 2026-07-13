#!/usr/bin/env python3
"""
Teron L-PFR Mock Server — glumi Teron fiskalni server za testiranje NTech-a.
Endpoint-i i format odgovora usklađeni sa Teron API dokumentacijom.
Port: 4566 (Teron standard)
"""

import json
import http.server
import re
import sys
import base64
import io
import os
import sqlite3
import time
from datetime import datetime, timezone, timedelta
from pathlib import Path

import socket
import urllib.parse

import qrcode
from receipt import generate_receipt, generate_receipt_html, generate_report, render_report_pdf, load_locale

# ── Konfiguracija ──────────────────────────────────────────
PORT  = 4566          # Teron standard port
HOST  = "0.0.0.0"
ESIR_ID = "NTECH001"  # naš 8-char ESIR identifikator

# Kartica emulator (NTech goroutine)
BE_HOST = os.environ.get("BE_HOST", "127.0.0.1")
BE_PORT = int(os.environ.get("BE_PORT", "4567"))
BE_PIN  = os.environ.get("BE_PIN", "1234")

DATA_DIR     = Path(__file__).parent / "data"
INVOICES_DIR = DATA_DIR / "invoices"
QR_DIR       = DATA_DIR / "qr"
RECEIPTS_DIR = DATA_DIR / "receipts"
LOG_FILE     = DATA_DIR / "server.log"
COUNTER_DIR  = DATA_DIR / "counters"
PERIOD_FILE  = DATA_DIR / "period_start.txt"

for d in [DATA_DIR, INVOICES_DIR, QR_DIR, RECEIPTS_DIR, COUNTER_DIR]:
    d.mkdir(parents=True, exist_ok=True)

# NTech SQLite baza (read-only) — fallback kad kartica emulator nije dostupan
NTECH_DB = os.environ.get("NTECH_SQLITE") or str(Path(__file__).parent.parent / "ntech.db")

def _ucitaj_verify_host():
    """Čita verify_host iz env, pa iz NTech SQLite baze."""
    if v := os.environ.get("VERIFY_HOST", ""):
        return v
    try:
        con = sqlite3.connect(f"file:{NTECH_DB}?mode=ro", uri=True)
        try:
            cur = con.execute("SELECT vrednost FROM podesavanja WHERE kljuc='verify_host'")
            row = cur.fetchone()
            return row[0] if row and row[0] else ""
        finally:
            con.close()
    except Exception:
        return ""

# Host za verifikacioni link na QR kodu (npr. "ntech.moja-firma.rs:3000").
# Ako je prazno, koristi se sandbox.suf.purs.gov.rs.
VERIFY_HOST = _ucitaj_verify_host()

def _ucitaj_fiskalni_pismo():
    """Čita fiskalni_pismo iz env var FISKALNI_PISMO ili iz NTech SQLite baze.
    Vrednosti: 'latin' (podrazumevano) ili 'cyrillic'."""
    if v := os.environ.get("FISKALNI_PISMO", ""):
        return v if v in ("latin", "cyrillic") else "latin"
    try:
        con = sqlite3.connect(f"file:{NTECH_DB}?mode=ro", uri=True)
        try:
            cur = con.execute("SELECT vrednost FROM podesavanja WHERE kljuc='fiskalni_pismo'")
            row = cur.fetchone()
            v = row[0] if row and row[0] else "latin"
            return v if v in ("latin", "cyrillic") else "latin"
        finally:
            con.close()
    except Exception:
        return "latin"

# Pismo fiskalnog računa: 'latin' ili 'cyrillic'
FISKALNI_PISMO = _ucitaj_fiskalni_pismo()


_be_pin_verifikovan = False


def _osiguraj_be_pin() -> bool:
    """Verifikuje PIN kod kartica emulatora ako to još nije učinjeno u ovom
    procesu — emulator (NTech, internal/be/kartica.go) posle be#34 odbija
    "sign" dok se prethodno ne pozove "verify_pin". pinUnesen je stanje na
    deljenoj Kartica instanci (globalno za sve TCP konekcije), pa je dovoljno
    da se verifikacija uspešno izvrši jednom po životnom veku NTech procesa."""
    global _be_pin_verifikovan
    if _be_pin_verifikovan:
        return True
    resp = be_command({"command": "verify_pin", "pin": BE_PIN})
    _be_pin_verifikovan = resp.get("status") == "ok"
    if not _be_pin_verifikovan:
        log(f"  ⚠️  automatska PIN verifikacija ka kartica emulatoru nije uspela: {resp}")
    return _be_pin_verifikovan


def be_command(cmd: dict) -> dict:
    """Šalje JSON komandu kartica emulatoru (NTech TCP :4567) i vraća odgovor."""
    try:
        with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
            s.settimeout(3)
            s.connect((BE_HOST, BE_PORT))
            s.sendall((json.dumps(cmd) + "\n").encode("utf-8"))
            buf = b""
            while True:
                chunk = s.recv(4096)
                if not chunk:
                    break
                buf += chunk
                if b"\n" in buf:
                    break
        return json.loads(buf.decode("utf-8").strip())
    except Exception as e:
        log(f"  ⚠️  be_command({cmd.get('command')}) greška: {e}")
        return {"status": "error", "message": str(e)}


def build_vl(full_data):
    """Gradi base64-kodirani payload za vl parametar verifikacionog URL-a."""
    payload = {
        "n":  full_data.get("invoiceNumber", ""),
        "ic": full_data.get("invoiceCounter", ""),
        "t":  full_data.get("sdcDateTime", ""),
        "a":  full_data.get("totalAmount", 0),
        "c":  full_data.get("tin", ""),
        "co": full_data.get("company", ""),
        "lo": full_data.get("store", ""),
        "ad": full_data.get("address", ""),
        "g":  full_data.get("city", ""),
        "di": full_data.get("district", ""),
        "it": full_data.get("invoiceType", "Normal"),
        "tr": full_data.get("transactionType", "Sale"),
        "tx": full_data.get("taxItems", []),
        "pm": full_data.get("payments", []),
        "ca": full_data.get("cashier", ""),
        "bi": full_data.get("buyerId", ""),
        "items": full_data.get("items", []),
    }
    j = json.dumps(payload, ensure_ascii=False, separators=(",", ":"))
    return base64.b64encode(j.encode("utf-8")).decode("ascii")

# ── Firma ───────────────────────────────────────────────────

def ucitaj_firmu():
    """Čita podatke o firmi iz NTech baze."""
    podaci = {}
    try:
        con = sqlite3.connect(f"file:{NTECH_DB}?mode=ro", uri=True)
        try:
            cur = con.execute(
                "SELECT kljuc, vrednost FROM podesavanja WHERE kljuc IN "
                "('naziv_firme','pib','maticni_broj','adresa','telefon',"
                "'poslovna_jedinica_naziv','poslovna_jedinica_oznaka','opstina','grad')"
            )
            podaci = {k: v for k, v in cur.fetchall()}
        finally:
            con.close()
    except Exception as e:
        log(f"  ⚠️  Ne mogu da pročitam firmu iz baze ({NTECH_DB}): {e}")

    naziv = podaci.get("naziv_firme") or "Test Company DOO"
    pib   = podaci.get("pib") or "123456789"
    return {
        "name":           naziv,
        "tin":            f"RS{pib}",   # Teron koristi RS prefiks
        "tinPlain":       pib,
        "mb":             podaci.get("maticni_broj") or "12345678",
        "address":        podaci.get("adresa") or "Test Address 1",
        "telefon":        podaci.get("telefon") or "",
        "locationName":   podaci.get("poslovna_jedinica_naziv") or naziv,
        "businessUnitId": podaci.get("poslovna_jedinica_oznaka") or "BU-001",
        "district":       podaci.get("opstina") or "Savski Venac",
        "city":           podaci.get("grad") or "Beograd",
    }

# ── Brojači ─────────────────────────────────────────────────

def get_counter(tip="total"):
    """Čita i inkrementira brojač za dati tip (total, pp, pr, ap, ar, kp, op, itd.)."""
    f = COUNTER_DIR / f"{tip}.txt"
    num = int(f.read_text().strip()) if f.exists() else 1
    f.write_text(str(num + 1))
    return num

def peek_counter(tip="total"):
    """Čita brojač bez inkrementiranja."""
    f = COUNTER_DIR / f"{tip}.txt"
    num = int(f.read_text().strip()) if f.exists() else 1
    return max(1, num - 1)

def counter_ext(invoice_type, transaction_type):
    """Vraća sufiks tipa transakcije (ПП, ПР, АП...) i ključ brojača."""
    t = (str(invoice_type).lower(), str(transaction_type).lower())
    mapping = {
        ("normal",   "sale"):   ("ПП", "pp"),
        ("normal",   "refund"): ("ПР", "pr"),
        ("advance",  "sale"):   ("АП", "ap"),
        ("advance",  "refund"): ("АР", "ar"),
        ("copy",     "sale"):   ("КП", "kp"),
        ("copy",     "refund"): ("КР", "kr"),
        ("training", "sale"):   ("ОП", "op"),
        ("training", "refund"): ("ОР", "or"),
    }
    return mapping.get(t, ("НН", "other"))

# ── Logging ─────────────────────────────────────────────────

def log(msg):
    now = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    line = f"[{now}] {msg}"
    print(line, flush=True)
    with open(LOG_FILE, "a", encoding="utf-8") as f:
        f.write(line + "\n")

# ── QR ──────────────────────────────────────────────────────

def generate_qr(url):
    img = qrcode.make(url)
    buf = io.BytesIO()
    img.save(buf, format="PNG")
    return base64.b64encode(buf.getvalue()).decode("utf-8")

# ── Vreme ───────────────────────────────────────────────────

def sada():
    tz = timezone(timedelta(hours=2))
    return datetime.now(tz).strftime("%Y-%m-%dT%H:%M:%S.000+02:00")

# ── PDV obračun ─────────────────────────────────────────────

# Teron koristi: Ж (20%), Ђ (10%), Е (posebna), А (0% neobveznici),
#                Г (oslobođen), З (0% bez prava odbitka)
# + starije oznake generičkog L-PFR-a za kompatibilnost
TAX_RATES = {
    # Teron oznake
    "Ж": 20.0,  # opšta stopa 20%
    "Ђ": 10.0,  # snižena stopa 10%
    "Е": 10.0,  # posebna snižena stopa
    "А":  0.0,  # neobveznici PDV-a
    "Г":  0.0,  # oslobođen bez prava na odbitak
    "З":  0.0,  # nije predmet oporezivanja
    # Generičke oznake (kompatibilnost)
    "Б": 20.0, "B": 20.0,
    "В":  0.0, "V":  0.0,
    "Д":  0.0, "D":  0.0,
    "A":  0.0, "G":  0.0, "E": 10.0,
}

def izracunaj_pdv(items):
    """Grupiše stavke po poreskoj oznaci i izračunava PDV iz bruto iznosa.
    Formula: pdv = bruto * stopa / (100 + stopa)"""
    grupe = {}
    for item in items:
        total = float(item.get("totalAmount", 0))
        for label in item.get("labels", []):
            rate = TAX_RATES.get(label, 0.0)
            if label not in grupe:
                grupe[label] = {"label": label, "rate": rate, "amount": 0.0}
            if rate > 0:
                grupe[label]["amount"] += total * rate / (100 + rate)
    return [
        {
            "label": d["label"],
            "categoryName": "PDV",
            "categoryType": 0,
            "rate": d["rate"],
            "amount": round(d["amount"], 4),
        }
        for d in grupe.values()
    ]

# ── Response handleri ───────────────────────────────────────

def resp_attention():
    return {"sdcDateTime": sada(), "status": "OK"}

def resp_status():
    st = be_command({"command": "status"})
    cert = be_command({"command": "certificate"})
    total = st.get("total_counter", 0)
    jid = cert.get("jid", "UNKNOWN")
    tin = cert.get("tin", "RS000000000")
    last_num = f"{ESIR_ID}-{jid}-{total}" if total >= 1 else ""
    return {
        "isPinRequired": st.get("pin_required", False),
        "auditRequired": False,
        "sdcDateTime": sada(),
        "lastInvoiceNumber": last_num,
        "protocolVersion": "1.0.0",
        "serialNumber": ESIR_ID,
        "tin": tin,
    }

def resp_verify_pin(request_body=None):
    uneti = ""
    if isinstance(request_body, dict):
        uneti = str(request_body.get("pin", "")).strip()
    elif isinstance(request_body, str):
        uneti = request_body.strip().strip('"')
    resp = be_command({"command": "verify_pin", "pin": uneti})
    if resp.get("status") == "ok":
        log("  🔓 PIN ispravan")
        return {"status": "OK", "message": "PIN verifikovan"}
    log("  🔒 Pogrešan PIN")
    return {"status": "ERROR", "code": resp.get("code", "2100"), "message": resp.get("message", "Pogrešan PIN")}

def resp_settings_get():
    return {
        "printerType": "Thermal",
        "printerInterface": "None",
        "lpfrEnabled": False,
        "vpfrEnabled": False,
        "authorizeLocalClients": False,
        "authorizeRemoteClients": False,
        "apiKey": "mock-api-key-0000",
        "webserverAddress": f"http://127.0.0.1:{PORT}/",
    }

def resp_settings_post():
    return {"status": "OK", "message": "Podešavanja sačuvana"}

def resp_certificate():
    c = be_command({"command": "certificate"})
    return {
        "serialNumber": c.get("jid", ESIR_ID),
        "tin":          c.get("tin", "RS000000000"),
        "name":         c.get("name", ""),
        "validFrom":    c.get("valid_from", "2024-01-01T00:00:00+01:00"),
        "validTo":      c.get("valid_to",   "2027-01-01T00:00:00+01:00"),
        "issuer":       c.get("issuer", "Poreska uprava RS"),
    }

def _build_invoice_response(req, request_id):
    """Gradi Teron odgovor za fiskalni račun."""
    # Teron zahtev dolazi unutar invoiceRequest omotača
    inv_req = req.get("invoiceRequest", req)

    invoice_type    = inv_req.get("invoiceType", "Normal")
    transaction_type = inv_req.get("transactionType", "Sale")
    items = inv_req.get("items", [])

    # PDV i ukupan iznos
    tax_items   = izracunaj_pdv(items)
    total_amount = round(sum(float(i.get("totalAmount", 0)) for i in items), 2)
    total_tax   = round(sum(t["amount"] for t in tax_items), 2)

    # Kartica emulator: podatke firme i potpis/brojače
    cert = be_command({"command": "certificate"})
    _osiguraj_be_pin()
    sign = be_command({
        "command":          "sign",
        "invoice_type":     invoice_type,
        "transaction_type": transaction_type,
        "total_amount":     total_amount,
    })

    if sign.get("status") == "blocked":
        raise RuntimeError(f"Kartica blokirana: {sign.get('message')}")
    if sign.get("status") != "ok":
        # npr. "error"/2101 (PIN nije verifikovan) — ne sme tiho da propadne u
        # podrazumevane vrednosti counter=1 (v. BUG.md #34), jer bi to izdalo
        # fiskalni broj koji se ponavlja iz računa u račun.
        raise RuntimeError(f"Kartica potpis nije uspeo: {sign.get('message', sign.get('status'))}")

    jid          = cert.get("jid", ESIR_ID)
    total_cnt    = sign.get("counter", 1)
    type_cnt     = sign.get("type_counter", 1)
    ext          = sign.get("counter_extension", "ПП")

    invoice_number   = f"{ESIR_ID}-{jid}-{total_cnt}"
    invoice_counter  = f"{type_cnt}/{total_cnt}{ext}"

    # firma podaci sa kartice
    firma = {
        "tinPlain":    cert.get("tin_plain", "000000000"),
        "tin":         cert.get("tin", "RS000000000"),
        "name":        cert.get("name", "Test Company DOO"),
        "locationName": cert.get("location_name", cert.get("name", "Test Company DOO")),
        "address":     cert.get("address", "Test Adresa 1"),
        "city":        cert.get("city", "Beograd"),
        "district":    cert.get("district", "Savski Venac"),
    }

    # Verifikacioni URL i QR kod
    if VERIFY_HOST:
        vl_payload = {
            "n":  invoice_number,
            "ic": invoice_counter,
            "t":  sada(),
            "a":  total_amount,
            "c":  firma["tinPlain"],
            "co": firma["name"],
            "lo": firma["locationName"],
            "ad": firma["address"],
            "g":  firma["city"],
            "di": firma["district"],
            "it": invoice_type,
            "tr": transaction_type,
            "tx": tax_items,
            "pm": inv_req.get("payment", [{"type": "Cash", "amount": total_amount}]),
            "ca": inv_req.get("cashier", "Kasir"),
            "bi": inv_req.get("buyerId", ""),
            "items": items,
        }
        vl = base64.b64encode(
            json.dumps(vl_payload, ensure_ascii=False, separators=(",", ":")).encode("utf-8")
        ).decode("ascii")
        scheme = "https" if VERIFY_HOST.startswith("https://") else "http"
        host = VERIFY_HOST.removeprefix("https://").removeprefix("http://").rstrip("/")
        verification_url = f"{scheme}://{host}/v/?vl={urllib.parse.quote(vl, safe='')}"
    else:
        verification_url = f"https://sandbox.suf.purs.gov.rs/v/?vl={invoice_number}"
    qr_b64 = generate_qr(verification_url)

    # Odgovor koji ide ka NTech-u (ESIR-u)
    odgovor = {
        "requestedBy":          ESIR_ID,
        "signedBy":             jid,
        "sdcDateTime":          sada(),
        "invoiceCounter":       invoice_counter,
        "invoiceCounterExtension": ext,
        "invoiceNumber":        invoice_number,
        "verificationUrl":      verification_url,
        "verificationQRCode":   qr_b64,
        "taxItems":             tax_items,
        "totalAmount":          total_amount,
        "totalTax":             total_tax,
        "messages":             "Success",
    }

    # povraćaj — ako je primljeno više nego što je duženo (npr. kupac dao krupniju
    # novčanicu), razlika se ispisuje na računu; kod refundacije nema povraćaja
    payments = inv_req.get("payment", [{"type": "Cash", "amount": total_amount}])
    total_paid = round(sum(float(p.get("amount", 0)) for p in payments), 2)
    refund = round(total_paid - total_amount, 2) if transaction_type != "Refund" else 0
    if refund < 0:
        refund = 0

    # Puni podaci za snimanje i generisanje računa
    full_data = {
        **odgovor,
        "requestId":     request_id,
        "invoiceType":   invoice_type,
        "transactionType": transaction_type,
        "items":         items,
        "payments":      payments,
        "cashier":       inv_req.get("cashier", "Kasir"),
        "buyerId":       inv_req.get("buyerId", ""),
        "referentDocumentNumber": inv_req.get("referentDocumentNumber", ""),
        "isFiscal":      invoice_type not in ("Copy", "Training", "Proforma"),
        "tin":           firma["tinPlain"],
        "company":       firma["name"],
        "store":         firma["locationName"],
        "address":       firma["address"],
        "district":      firma["district"],
        "refund":        refund,
        # za avansni konačni
        "advancePaid":   req.get("advancePaid", 0),
        "advanceTax":    req.get("advanceTax", 0),
    }

    # Snimi JSON
    inv_path = INVOICES_DIR / f"{total_cnt:06d}_{request_id}.json"
    with open(inv_path, "w", encoding="utf-8") as fh:
        json.dump(full_data, fh, indent=2, ensure_ascii=False)

    # Snimi QR PNG
    qr_path = QR_DIR / f"{total_cnt:06d}_{request_id}.png"
    with open(qr_path, "wb") as fh:
        fh.write(base64.b64decode(qr_b64))

    # Generiši tekst i HTML račun (pismo određuje podešavanje fiskalni_pismo)
    receipt_text = generate_receipt(full_data, FISKALNI_PISMO)
    txt_path = RECEIPTS_DIR / f"{total_cnt:06d}_{request_id}.txt"
    txt_path.write_text(receipt_text, encoding="utf-8")

    html_txt = generate_receipt_html(full_data, FISKALNI_PISMO)
    html_path = RECEIPTS_DIR / f"{total_cnt:06d}_{request_id}.html"
    html_path.write_text(html_txt, encoding="utf-8")

    # Dodaj journal (tekst računa) u odgovor
    odgovor["journal"] = receipt_text

    log(f"  🧾 {invoice_number} | {ext} | {total_amount:.2f} din | PDV {total_tax:.2f}")
    return odgovor

def resp_invoice(request_id, request_body=None):
    if not request_body:
        return {"error": "Telo zahteva je obavezno"}, 400
    return _build_invoice_response(request_body, request_id)

def resp_invoice_final(request_id, request_body=None):
    """Konačni račun koji zatvara avanse (/api/invoices/final)."""
    if not request_body:
        return {"error": "Telo zahteva je obavezno"}, 400
    return _build_invoice_response(request_body, request_id)

def resp_invoice_last():
    """Vraća poslednji sačuvani račun."""
    files = sorted(INVOICES_DIR.glob("*.json"), reverse=True)
    if not files:
        return {"error": "Nema računa"}, 404
    try:
        return json.loads(files[0].read_text(encoding="utf-8"))
    except Exception:
        return {"error": "Greška pri čitanju računa"}, 500

def resp_invoice_by_request(request_id):
    for f in INVOICES_DIR.glob("*.json"):
        try:
            data = json.loads(f.read_text(encoding="utf-8"))
            if data.get("requestId") == request_id:
                return data
        except Exception:
            continue
    return {"error": f"Račun {request_id} nije pronađen"}, 404

def resp_invoice_by_number(invoice_number):
    for f in INVOICES_DIR.glob("*.json"):
        try:
            data = json.loads(f.read_text(encoding="utf-8"))
            if data.get("invoiceNumber") == invoice_number:
                return data
        except Exception:
            continue
    return {"error": f"Račun {invoice_number} nije pronađen"}, 404

def resp_invoice_search(request_body=None):
    """Osnovna pretraga — vraća CSV."""
    invoices = []
    for f in sorted(INVOICES_DIR.glob("*.json")):
        try:
            data = json.loads(f.read_text(encoding="utf-8"))
            invoices.append(data)
        except Exception:
            continue
    lines = [
        f"{d['invoiceNumber']},{d.get('invoiceType','Normal')},{d.get('transactionType','Sale')},{d.get('sdcDateTime','')},{d.get('totalAmount',0)}"
        for d in invoices
    ]
    return "\n".join(lines)

# ── Dnevni pazar / presek stanja ───────────────────────────

def get_period_start():
    """Vreme poslednjeg preseka stanja; ako ga još nema, postavlja se na sada."""
    if PERIOD_FILE.exists():
        return PERIOD_FILE.read_text().strip()
    ts = sada()
    PERIOD_FILE.write_text(ts)
    return ts

def reset_period():
    """Zatvara tekući period (presek stanja) — sledeći GET summary počinje od sada."""
    PERIOD_FILE.write_text(sada())

def compute_summary(from_iso=None, to_iso=None):
    """Sabira promet iz sačuvanih fiskalnih računa u zadatom periodu.
    Bez argumenata: od poslednjeg preseka stanja (get_period_start) do sada."""
    start = from_iso or get_period_start()
    end = to_iso or sada()

    total = 0.0
    total_cash = 0.0
    count = 0
    by_tax = {}
    by_cashier = {}
    by_payment = {}
    by_article = {}
    by_article_advance = {}

    for f in sorted(INVOICES_DIR.glob("*.json")):
        try:
            inv = json.loads(f.read_text(encoding="utf-8"))
        except Exception:
            continue
        t = inv.get("sdcDateTime", "")
        if not t or not (start <= t <= end) or not inv.get("isFiscal", True):
            continue
        count += 1

        amount = float(inv.get("totalAmount", 0))
        is_refund = inv.get("transactionType") == "Refund"
        znak = -1 if is_refund else 1
        total += znak * amount

        for p in inv.get("payments", []):
            ptype = p.get("paymentType") or p.get("type") or "Other"
            pamt = znak * float(p.get("amount", 0))
            by_payment[ptype] = by_payment.get(ptype, 0.0) + pamt
            if ptype == "Cash":
                total_cash += pamt

        kasir = inv.get("cashier", "Kasir")
        by_cashier[kasir] = by_cashier.get(kasir, 0.0) + znak * amount

        for ti in inv.get("taxItems", []):
            label = ti.get("label", "")
            if label not in by_tax:
                by_tax[label] = {"label": label, "rate": ti.get("rate", 0),
                                  "category": "VAT" if ti.get("rate", 0) > 0 else "N-TAX",
                                  "amount": 0.0, "osnovica": 0.0}
            by_tax[label]["amount"] += znak * float(ti.get("amount", 0))

        cilj = by_article_advance if inv.get("invoiceType") == "Advance" else by_article
        for item in inv.get("items", []):
            naziv = item.get("name", "")
            if naziv not in cilj:
                cilj[naziv] = {"articleName": naziv, "gtin": None, "plu": None,
                                "taxLabel": (item.get("labels") or [""])[0],
                                "amount": 0.0, "quantity": 0.0}
            cilj[naziv]["amount"] += znak * float(item.get("totalAmount", 0))
            cilj[naziv]["quantity"] += znak * float(item.get("quantity", 0))
            for label in item.get("labels", []):
                if label in by_tax:
                    by_tax[label]["osnovica"] += znak * float(item.get("totalAmount", 0))

    return {
        "startOfPeriod": start,
        "endOfPeriod": end,
        "invoiceCount": count,
        "total": round(total, 2),
        "totalCash": round(total_cash, 2),
        "totalByTax": [
            {"amount": round(v["amount"], 4), "category": v["category"], "label": v["label"],
             "rate": v["rate"], "osnovica": round(v["osnovica"], 2)}
            for v in by_tax.values()
        ],
        "totalByCashier": [{"amount": round(v, 2), "name": k} for k, v in by_cashier.items()],
        "totalByPaymentType": [{"amount": round(v, 2), "paymentType": k} for k, v in by_payment.items()],
        "totalByArticle": [
            {**v, "amount": round(v["amount"], 2)} for v in by_article.values()
        ],
        "totalByArticleAdvance": [
            {**v, "amount": round(v["amount"], 2)} for v in by_article_advance.values()
        ],
    }

def resp_financial_summary_get():
    return compute_summary()

def resp_financial_summary_delete():
    reset_period()
    log("  📊 Presek stanja urađen — brojači prometa resetovani")
    return ("", 204)

def resp_financial_report_summary(request_body=None):
    body = request_body or {}
    from_date = body.get("fromDate")
    to_date = body.get("toDate")
    from_iso = f"{from_date}T00:00:00.000+02:00" if from_date else None
    to_iso = f"{to_date}T23:59:59.999+02:00" if to_date else None
    summary = compute_summary(from_iso, to_iso)

    cert = be_command({"command": "certificate"})
    firma = ucitaj_firmu()
    lang = "cyrillic" if str(body.get("language", "")).lower().startswith("sr-cyrl") else "latin"
    title = "ДНЕВНИ ИЗВЕШТАЈ" if lang == "cyrillic" else "DNEVNI IZVEŠTAJ"

    report_data = {
        "title": title,
        "number": get_counter("report"),
        "dateTime": sada(),
        "tin": firma["tinPlain"],
        "businessName": firma["name"],
        "locationName": firma["locationName"],
        "address": firma["address"],
        "district": firma["district"],
        "uid": cert.get("jid", ESIR_ID),
        "startDate": (from_date or summary["startOfPeriod"][:10]),
        "endDate": (to_date or summary["endOfPeriod"][:10]),
        "total": {
            "invoiceCount": summary["invoiceCount"],
            "payments": [{"paymentType": p["paymentType"], "amount": p["amount"]} for p in summary["totalByPaymentType"]],
            "totalPayments": summary["total"],
            "taxItems": [{"label": t["label"], "rate": t["rate"], "total": t["osnovica"], "amount": t["amount"]} for t in summary["totalByTax"]],
            "totalTax": round(sum(t["amount"] for t in summary["totalByTax"]), 2),
        },
        "perTransactionType": [],
    }
    tekst = generate_report(report_data, lang)
    pdf_bytes = render_report_pdf(tekst)
    pdf_b64 = base64.b64encode(pdf_bytes).decode("ascii")
    filename = f"{title} - {report_data['startDate']} - {report_data['endDate']}.pdf"
    log(f"  📄 Dnevni izveštaj #{report_data['number']} generisan ({summary['invoiceCount']} računa)")
    return {"reportPdfBase64": pdf_b64, "reportName": title, "filename": filename}

# ── Rute ────────────────────────────────────────────────────

ROUTES = [
    ("GET",  "api/attention",                   "attention"),
    ("GET",  "api/status",                      "status"),
    ("POST", "api/pin",                         "verify_pin"),
    ("GET",  "api/settings",                    "settings_get"),
    ("POST", "api/settings",                    "settings_post"),
    ("GET",  "api/certificate",                 "certificate"),
    ("POST", "api/invoices/final",              "invoice_final"),
    ("GET",  "api/invoices/last",               "invoice_last"),
    ("GET",  "api/invoices/request/:requestId", "invoice_by_request"),
    ("GET",  "api/invoices/:invoiceNumber",     "invoice_by_number"),
    ("POST", "api/invoices/search",             "invoice_search"),
    ("POST", "api/invoices",                    "invoice"),
    ("GET",  "api/financial/summary",           "financial_summary_get"),
    ("DELETE", "api/financial/summary",         "financial_summary_delete"),
    ("POST", "api/financial/report/summary",    "financial_report_summary"),
]

HANDLERS = {
    "attention":               resp_attention,
    "status":                  resp_status,
    "verify_pin":              resp_verify_pin,
    "settings_get":            resp_settings_get,
    "settings_post":           resp_settings_post,
    "certificate":             resp_certificate,
    "invoice":                 resp_invoice,
    "invoice_final":           resp_invoice_final,
    "invoice_last":            resp_invoice_last,
    "invoice_by_request":      resp_invoice_by_request,
    "invoice_by_number":       resp_invoice_by_number,
    "invoice_search":          resp_invoice_search,
    "financial_summary_get":   resp_financial_summary_get,
    "financial_summary_delete": resp_financial_summary_delete,
    "financial_report_summary": resp_financial_report_summary,
}

# ── HTTP Handler ─────────────────────────────────────────────

class FiscalHandler(http.server.BaseHTTPRequestHandler):

    def log_message(self, fmt, *args):
        pass  # koristimo naš log

    def do_GET(self):    self._handle("GET")
    def do_POST(self):   self._handle("POST")
    def do_DELETE(self): self._handle("DELETE")

    def do_OPTIONS(self):
        self.send_response(200)
        self._cors()
        self.end_headers()

    def _cors(self):
        self.send_header("Access-Control-Allow-Origin", "*")
        self.send_header("Access-Control-Allow-Methods", "GET,POST,DELETE,OPTIONS")
        self.send_header("Access-Control-Allow-Headers", "Content-Type, Authorization")

    def _read_body(self):
        length = int(self.headers.get("Content-Length", 0))
        if length == 0:
            return None
        raw = self.rfile.read(length).decode("utf-8")
        try:
            return json.loads(raw)
        except Exception:
            return raw

    def _handle(self, method):
        path = self.path.split("?")[0].strip("/")
        status = 404
        body = {"error": "Not Found", "path": path}
        content_type = "application/json; charset=utf-8"
        matched = None

        for r_method, r_pattern, r_name in ROUTES:
            if r_method != method:
                continue
            regex = "^" + re.sub(r":(\w+)", r"(?P<\1>[^/]+)", r_pattern) + "$"
            m = re.match(regex, path)
            if not m:
                continue

            matched = r_name
            handler = HANDLERS.get(r_name)
            params  = m.groupdict()
            request_id = self.headers.get("RequestId", f"mock-{int(time.time())}")

            # Poziv handlera
            if r_name in ("invoice", "invoice_final"):
                result = handler(request_id, self._read_body())
            elif r_name == "verify_pin":
                result = handler(self._read_body())
            elif r_name == "invoice_by_request":
                result = handler(params.get("requestId", ""))
            elif r_name == "invoice_by_number":
                result = handler(params.get("invoiceNumber", ""))
            elif r_name == "invoice_search":
                result = handler(self._read_body())
            elif r_name == "financial_report_summary":
                result = handler(self._read_body())
            elif r_name in ("settings_post",):
                self._read_body()
                result = handler()
            else:
                result = handler()

            # Razdvoji (body, status) ako handler vratio tuple
            if isinstance(result, tuple):
                body, status = result
            else:
                body   = result
                status = 200
            break

        # Serializacija
        if r_name == "invoice_search" and isinstance(body, str):
            resp_bytes = body.encode("utf-8")
            content_type = "text/csv; charset=utf-8"
        elif isinstance(body, (dict, list)):
            resp_bytes = json.dumps(body, indent=2, ensure_ascii=False).encode("utf-8")
        else:
            resp_bytes = str(body).encode("utf-8")

        emoji = "✅" if status == 200 else "❌"
        log(f"  {emoji} {method} /{path} → {matched or '404'} [{status}]")

        self.send_response(status)
        self._cors()
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(resp_bytes)))
        self.end_headers()
        self.wfile.write(resp_bytes)

# ── Main ─────────────────────────────────────────────────────

def main():
    cert = be_command({"command": "certificate"})
    jid  = cert.get("jid", "?")
    tin  = cert.get("tin_plain", "?")
    name = cert.get("name", "?")
    log("╔══════════════════════════════════════════════════╗")
    log("║   🧾  Teron L-PFR Mock Server                  ║")
    log(f"║   http://{HOST}:{PORT}/                      ║")
    log("╚══════════════════════════════════════════════════╝")
    log(f"  📁 Podaci:   {DATA_DIR}")
    log(f"  🧾 Računi:   {INVOICES_DIR}")
    log(f"  📱 QR PNG:   {QR_DIR}")
    log(f"  📝 Log:      {LOG_FILE}")
    log(f"  🏢 Firma:    {name} | PIB: {tin}")
    log(f"  🆔 ESIR ID:  {ESIR_ID}  |  BE JID: {jid}  (kartica: {BE_HOST}:{BE_PORT})")
    log("  ▶  Server pokrenut. Ctrl+C za gašenje.")

    server = http.server.HTTPServer((HOST, PORT), FiscalHandler)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        log("  ⏹  Server zaustavljen.")
        server.server_close()

if __name__ == "__main__":
    main()
