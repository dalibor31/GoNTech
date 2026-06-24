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

import qrcode
from receipt import generate_receipt, generate_receipt_html, generate_report, load_locale

# ── Konfiguracija ──────────────────────────────────────────
PORT  = 4566          # Teron standard port
HOST  = "0.0.0.0"
ESIR_ID = "NTECH001"  # naš 8-char ESIR identifikator
BE_ID   = "TRNMOCK1"  # simulirani BE/LPFR identifikator

DATA_DIR     = Path(__file__).parent / "data"
INVOICES_DIR = DATA_DIR / "invoices"
QR_DIR       = DATA_DIR / "qr"
RECEIPTS_DIR = DATA_DIR / "receipts"
LOG_FILE     = DATA_DIR / "server.log"
COUNTER_DIR  = DATA_DIR / "counters"

for d in [DATA_DIR, INVOICES_DIR, QR_DIR, RECEIPTS_DIR, COUNTER_DIR]:
    d.mkdir(parents=True, exist_ok=True)

# Test PIN (pravi Teron traži PIN za BE karticu)
PIN_BE = "1234"

# NTech SQLite baza (read-only) — čita podatke o firmi
NTECH_DB = os.environ.get("NTECH_SQLITE") or str(Path(__file__).parent.parent / "ntech.db")

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
    f = ucitaj_firmu()
    last = peek_counter("total")
    last_num = f"{ESIR_ID}-{BE_ID}-{last}" if last >= 1 else ""
    return {
        "isPinRequired": False,
        "auditRequired": False,
        "sdcDateTime": sada(),
        "lastInvoiceNumber": last_num,
        "protocolVersion": "1.0.0",
        "serialNumber": ESIR_ID,
        "tin": f["tin"],
    }

def resp_verify_pin(request_body=None):
    uneti = ""
    if isinstance(request_body, dict):
        uneti = str(request_body.get("pin", "")).strip()
    elif isinstance(request_body, str):
        uneti = request_body.strip().strip('"')
    if uneti == PIN_BE:
        log("  🔓 PIN ispravan")
        return {"status": "OK", "message": "PIN verifikovan"}
    log("  🔒 Pogrešan PIN")
    return {"status": "ERROR", "code": "2100", "message": "Pogrešan PIN"}

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
    f = ucitaj_firmu()
    return {
        "serialNumber": BE_ID,
        "tin": f["tin"],
        "name": f["name"],
        "validFrom": "2024-01-01T00:00:00+01:00",
        "validTo":   "2027-01-01T00:00:00+01:00",
        "issuer": "Poreska uprava RS",
    }

def _build_invoice_response(req, request_id):
    """Gradi Teron odgovor za fiskalni račun."""
    # Teron zahtev dolazi unutar invoiceRequest omotača
    inv_req = req.get("invoiceRequest", req)

    invoice_type    = inv_req.get("invoiceType", "Normal")
    transaction_type = inv_req.get("transactionType", "Sale")
    items = inv_req.get("items", [])

    # Brojači
    total_cnt = get_counter("total")
    ext, tip_key = counter_ext(invoice_type, transaction_type)
    type_cnt  = get_counter(tip_key)

    invoice_number   = f"{ESIR_ID}-{BE_ID}-{total_cnt}"
    invoice_counter  = f"{type_cnt}/{total_cnt}{ext}"
    verification_url = f"https://sandbox.suf.purs.gov.rs/v/?vl={invoice_number}"
    qr_b64 = generate_qr(verification_url)

    # PDV i ukupan iznos
    tax_items   = izracunaj_pdv(items)
    total_amount = round(sum(float(i.get("totalAmount", 0)) for i in items), 2)
    total_tax   = round(sum(t["amount"] for t in tax_items), 2)

    firma = ucitaj_firmu()

    # Odgovor koji ide ka NTech-u (ESIR-u)
    odgovor = {
        "requestedBy":          ESIR_ID,
        "signedBy":             BE_ID,
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

    # Puni podaci za snimanje i generisanje računa
    full_data = {
        **odgovor,
        "requestId":     request_id,
        "invoiceType":   invoice_type,
        "transactionType": transaction_type,
        "items":         items,
        "payments":      inv_req.get("payment", [{"type": "Cash", "amount": total_amount}]),
        "cashier":       inv_req.get("cashier", "Kasir"),
        "buyerId":       inv_req.get("buyerId", ""),
        "referentDocumentNumber": inv_req.get("referentDocumentNumber", ""),
        "isFiscal":      invoice_type not in ("Copy", "Training", "Proforma"),
        "tin":           firma["tinPlain"],
        "company":       firma["name"],
        "store":         firma["locationName"],
        "address":       firma["address"],
        "district":      firma["district"],
        "refund":        0,
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

    # Generiši tekst i HTML račun
    receipt_text = generate_receipt(full_data, "latin")
    txt_path = RECEIPTS_DIR / f"{total_cnt:06d}_{request_id}.txt"
    txt_path.write_text(receipt_text, encoding="utf-8")

    html_txt = generate_receipt_html(full_data, "latin")
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
]

HANDLERS = {
    "attention":          resp_attention,
    "status":             resp_status,
    "verify_pin":         resp_verify_pin,
    "settings_get":       resp_settings_get,
    "settings_post":      resp_settings_post,
    "certificate":        resp_certificate,
    "invoice":            resp_invoice,
    "invoice_final":      resp_invoice_final,
    "invoice_last":       resp_invoice_last,
    "invoice_by_request": resp_invoice_by_request,
    "invoice_by_number":  resp_invoice_by_number,
    "invoice_search":     resp_invoice_search,
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
    firma = ucitaj_firmu()
    log("╔══════════════════════════════════════════════════╗")
    log("║   🧾  Teron L-PFR Mock Server                  ║")
    log(f"║   http://{HOST}:{PORT}/                      ║")
    log("╚══════════════════════════════════════════════════╝")
    log(f"  📁 Podaci:   {DATA_DIR}")
    log(f"  🧾 Računi:   {INVOICES_DIR}")
    log(f"  📱 QR PNG:   {QR_DIR}")
    log(f"  📝 Log:      {LOG_FILE}")
    log(f"  🏢 Firma:    {firma['name']} | PIB: {firma['tinPlain']}")
    log(f"  🆔 ESIR ID:  {ESIR_ID}  |  BE ID: {BE_ID}")
    log(f"  🔑 Test PIN: {PIN_BE}")
    log("  ▶  Server pokrenut. Ctrl+C za gašenje.")

    server = http.server.HTTPServer((HOST, PORT), FiscalHandler)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        log("  ⏹  Server zaustavljen.")
        server.server_close()

if __name__ == "__main__":
    main()
