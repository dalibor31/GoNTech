#!/usr/bin/env python3
"""
myLPFR Mock Server — Kompletan fiskalni server za testiranje
================================================================
Podržava sve /agent/v3, /api/v3 i /extension/v3 endpoint-e.
Automatski generiše QR kodove, snima račune, vodi log.
Pokreće se preko start.sh, gasi preko stop.sh.
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
PORT = 8989
HOST = "0.0.0.0"
DATA_DIR = Path(__file__).parent / "data"
INVOICES_DIR = DATA_DIR / "invoices"
QR_DIR = DATA_DIR / "qr"
RECEIPTS_DIR = DATA_DIR / "receipts"
LOG_FILE = DATA_DIR / "server.log"

# Inicijalizuj foldere
for d in [DATA_DIR, INVOICES_DIR, QR_DIR, RECEIPTS_DIR]:
    d.mkdir(parents=True, exist_ok=True)

# Prefix računa (čitaj iz fajla, ili kreni od 1)
COUNTER_FILE = DATA_DIR / "counter.txt"

# ── Bezbednosni element (mock kartica) ─────────────────────
# Fiksni test PIN — pravu karticu otključava korisnik svojim PIN-om,
# ovde je samo test vrednost kojom glumimo otključavanje.
PIN_BE = "1234"

# Putanja do NTech SQLite baze — čita se read-only. Podrazumevano ../ntech.db
# (koren repozitorijuma), može se promeniti preko NTECH_SQLITE.
NTECH_DB = os.environ.get("NTECH_SQLITE") or str(Path(__file__).parent.parent / "ntech.db")

def ucitaj_firmu():
    """Čita podatke o firmi iz NTech baze (read-only) i vraća ih kao dict.
    Bezbednosni element 'već zna' identitet poreskog obveznika — ovde to glumimo
    čitanjem profila firme iz tabele podesavanja. Ako baza ili ključ nedostaje,
    vraćamo test vrednosti da server i dalje radi."""
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
    return {
        "name":           naziv,
        "tin":            podaci.get("pib") or "123456789",
        "mb":             podaci.get("maticni_broj") or "12345678",
        "address":        podaci.get("adresa") or "Test Address 1",
        "telefon":        podaci.get("telefon") or "",
        "locationName":   podaci.get("poslovna_jedinica_naziv") or naziv,
        "businessUnitId": podaci.get("poslovna_jedinica_oznaka") or "BU-001",
        "district":       podaci.get("opstina") or "Savski Venac",
        "city":           podaci.get("grad") or "Beograd",
    }

def get_next_invoice_number():
    """Vraća i inkrementira broj računa."""
    if COUNTER_FILE.exists():
        num = int(COUNTER_FILE.read_text().strip())
    else:
        num = 1
    COUNTER_FILE.write_text(str(num + 1))
    return f"{num:06d}"

def log(msg):
    """Upisuje poruku u log fajl i na stdout."""
    now = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    line = f"[{now}] {msg}"
    print(line, flush=True)
    with open(LOG_FILE, "a", encoding="utf-8") as f:
        f.write(line + "\n")

def generate_qr(url):
    """Pravi QR kod PNG i vraća base64 string."""
    img = qrcode.make(url)
    buf = io.BytesIO()
    img.save(buf, format="PNG")
    return base64.b64encode(buf.getvalue()).decode("utf-8")

# ── Gradi rute iz OpenAPI speca ─────────────────────────────

# Ako postoji myLPFR-api-docs.json, koristi ga.
# Ako ne, koristi hardkodovane rute.
SPEC_FILE = Path(__file__).parent.parent / "myLPFR-api-docs.json"

# Default rute (pale su iz Swagger speca)
DEFAULT_ROUTES = [
    # Agent API
    ("GET",  "agent/v3/attention",              "attention"),
    ("GET",  "agent/v3/environment-parameters",  "environment"),
    ("POST", "agent/v3/invoices",                "invoice"),
    ("GET",  "agent/v3/invoices/:requestId",     "invoice_lookup"),
    ("POST", "agent/v3/open-drawer",             "open_drawer"),
    ("POST", "agent/v3/pin",                     "verify_pin"),
    ("POST", "agent/v3/print-text",              "print_text"),
    ("GET",  "agent/v3/receipts/:requestId",     "receipt"),
    ("GET",  "agent/v3/receipts/:requestId/text","receipt_text"),
    ("GET",  "agent/v3/receipts/:requestId/html","receipt_html"),
    ("GET",  "agent/v3/reports/daily",           "daily_report"),
    ("GET",  "agent/v3/reports/daily/text",      "daily_report_text"),
    ("GET",  "agent/v3/reports/periodic",        "periodic_report"),
    ("GET",  "agent/v3/reports/periodic/text",   "periodic_report_text"),
    ("GET",  "agent/v3/status",                  "status"),
    ("GET",  "agent/v3/subject",                 "subject"),
    # E-SDC API
    ("GET",  "api/v3/attention",                 "attention"),
    ("GET",  "api/v3/environment-parameters",     "environment"),
    ("POST", "api/v3/invoices",                  "invoice"),
    ("GET",  "api/v3/invoices/:requestId",       "invoice_lookup"),
    ("POST", "api/v3/pin",                       "verify_pin"),
    ("GET",  "api/v3/status",                    "status"),
    # Extension API
    ("GET",  "extension/v3/notifications",       "notifications"),
    ("GET",  "extension/v3/reports/daily",       "daily_report"),
    ("GET",  "extension/v3/reports/periodic",    "periodic_report"),
    ("GET",  "extension/v3/status-codes",        "status_codes"),
    ("GET",  "extension/v3/subject",             "subject"),
]

# ── Response handler-i ──────────────────────────────────────

def sada():
    """Trenutno vreme u ISO formatu sa +02:00."""
    tz = timezone(timedelta(hours=2))
    return datetime.now(tz).strftime("%Y-%m-%dT%H:%M:%S.000+02:00")

# Poreske stope po oznaci — cene su BRUTO (PDV uključen u totalAmount)
TAX_RATES = {
    "А": 0.0,  "A": 0.0,   # neobveznici PDV-a / oslobođen
    "Б": 20.0, "B": 20.0,  # opšta stopa 20%
    "В": 0.0,  "V": 0.0,   # oslobođen sa pravom odbitka
    "Г": 0.0,  "G": 0.0,   # oslobođen bez prava odbitka
    "Д": 0.0,  "D": 0.0,   # nije predmet oporezivanja
    "Ђ": 10.0,              # autorski honorar (snižena stopa)
    "Е": 10.0, "E": 10.0,  # posebna snižena stopa 10%
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

def resp_attention():
    return {"sdcDateTime": sada(), "status": "OK"}

def resp_status():
    return {
        "isPinRequired": True,
        "auditRequired": False,
        "sdcDateTime": sada(),
        "lastInvoiceNumber": get_last_invoice_number(),
        "protocolVersion": "1.0.0.0",
        "secureElementVersion": "1.0",
        "hardwareVersion": "1.0",
        "softwareVersion": "0.3.18",
        "deviceSerialNumber": "50-0002-NX6LC40XR3TQ",
        "make": "MyOffice DOO",
        "model": "myLPFR",
        "mssc": [],
        "gsc": ["1300", "0210"],
        "supportedLanguages": ["sr-Cyrl-RS", "sr-Latin-RS"],
        "uid": "",
        "taxCoreApi": "https://suf-sandbox.purs.gov.rs",
        "currentTaxRates": None,
        "allTaxRates": [],
    }

def resp_environment():
    f = ucitaj_firmu()
    return {
        "tin": f["tin"],
        "uid": "550e8400-e29b-41d4-a716-446655440000",
        "taxCoreApi": "https://suf-sandbox.purs.gov.rs",
        "sufVersion": "3.0",
        "supportedLanguages": ["sr-Cyrl-RS", "sr-Latin-RS"],
        "taxRates": [{
            "validFrom": "2026-01-01",
            "groupId": 1,
            "taxCategories": [{
                "categoryId": 1,
                "name": "PDV",
                "categoryType": "0",
                "orderId": 1,
                "taxRates": [
                    {"rateId": 1, "rate": 20.0, "label": "S"},
                    {"rateId": 2, "rate": 10.0, "label": "P"},
                ],
            }],
        }],
    }

def resp_subject():
    f = ucitaj_firmu()
    return {
        "tin": f["tin"],
        "mb": f["mb"],
        "uid": "550e8400-e29b-41d4-a716-446655440000",
        "name": f["name"],
        "address": f["address"],
        "city": f["city"],
        "country": "RS",
        "district": f["district"],
        "locationName": f["locationName"],
        "businessUnitId": f["businessUnitId"],
    }

def resp_invoice(request_id, request_body=None):
    invoice_number = get_next_invoice_number()
    verification_url = f"https://suf-sandbox.purs.gov.rs/verify/{request_id}"
    qr_b64 = generate_qr(verification_url)
    uid = f"550e8400-e29b-41d4-a716-{invoice_number.zfill(12)}"

    invoice_data = {
        "uid": uid,
        "requestId": request_id,
        "signedXml": f"<Invoice><UID>{uid}</UID><Number>{invoice_number}</Number><RequestId>{request_id}</RequestId><SignedAt>{sada()}</SignedAt></Invoice>",
        "sdcDateTime": sada(),
        "invoiceNumber": invoice_number,
        "verificationUrl": verification_url,
        "qrCode": qr_b64,
        "encryptedInternalData": f"ENC_{uid}",
        "signature": f"SIG_{invoice_number}_{request_id[:8]}",
    }

    # Proširi podatke iz tela zahteva (za štampu)
    full_data = dict(invoice_data)
    if request_body:
        full_data.update(request_body)
    full_data["invoiceNumber"] = invoice_number
    full_data["sdcDateTime"] = invoice_data["sdcDateTime"]
    full_data["qrCode"] = qr_b64
    full_data["isFiscal"] = full_data.get("isFiscal", True)
    f = ucitaj_firmu()
    full_data.setdefault("tin", f["tin"])
    full_data.setdefault("company", f["name"])
    full_data.setdefault("store", f["locationName"])
    full_data.setdefault("address", f["address"])
    full_data.setdefault("district", f["district"])
    full_data.setdefault("cashier", "Marko Marković")
    full_data.setdefault("transactionType", "NSX")
    items = full_data.get("items", [])
    total_amount = round(sum(float(item.get("totalAmount", 0)) for item in items), 2)
    full_data.setdefault("totalAmount", total_amount)
    full_data.setdefault("payments", [{"type": "Cash", "amount": full_data["totalAmount"]}])
    tax_items = izracunaj_pdv(items)
    full_data["taxItems"] = tax_items
    full_data["totalTax"] = round(sum(t["amount"] for t in tax_items), 2)
    full_data.setdefault("refund", 0)
    full_data.setdefault("invoiceType", "Normal")
    # Obogati odgovor koji ide ka ESIR-u (NTech-u)
    invoice_data["taxItems"] = tax_items
    invoice_data["totalAmount"] = full_data["totalAmount"]
    invoice_data["totalTax"] = full_data["totalTax"]
    invoice_data["messages"] = "Success"
    invoice_data["businessName"] = full_data.get("company", "")
    invoice_data["tin"] = full_data.get("tin", "")
    invoice_data["locationName"] = full_data.get("store", "")
    invoice_data["address"] = full_data.get("address", "")

    # Snimi kompletan račun
    invoice_path = INVOICES_DIR / f"{invoice_number}_{request_id}.json"
    with open(invoice_path, "w", encoding="utf-8") as f:
        json.dump(full_data, f, indent=2, ensure_ascii=False)

    # Snimi QR kod
    qr_path = QR_DIR / f"{invoice_number}_{request_id}.png"
    with open(qr_path, "wb") as f:
        f.write(base64.b64decode(qr_b64))

    # Generiši i snimi tekst računa (latinica)
    receipt_text = generate_receipt(full_data, "latin")
    receipt_path = RECEIPTS_DIR / f"{invoice_number}_{request_id}.txt"
    receipt_path.write_text(receipt_text, encoding="utf-8")

    # Generiši HTML račun
    receipt_html = generate_receipt_html(full_data, "latin")
    html_path = RECEIPTS_DIR / f"{invoice_number}_{request_id}.html"
    html_path.write_text(receipt_html, encoding="utf-8")

    log(f"  🧾 RAČUN {invoice_number} | requestId={request_id} | QR={qr_path.name} | Račun={receipt_path.name} | HTML={html_path.name}")
    return invoice_data

def resp_invoice_lookup(request_id):
    """Pronađi postojeći račun po requestId."""
    for f in INVOICES_DIR.glob("*.json"):
        try:
            data = json.loads(f.read_text(encoding="utf-8"))
            if data.get("requestId") == request_id:
                log(f"  🔍 Pronađen račun: {f.name}")
                return data
        except Exception:
            continue
    return None

def get_last_invoice_number():
    """Poslednji broj računa (bez inkrementiranja)."""
    if COUNTER_FILE.exists():
        num = int(COUNTER_FILE.read_text().strip()) - 1
        return f"{num:06d}" if num >= 1 else ""
    return ""

def resp_verify_pin(request_body=None):
    """Glumi otključavanje kartice PIN-om. Prihvata telo kao JSON {"pin": "..."}
    ili kao goli string. Poredi sa fiksnim test PIN-om PIN_BE."""
    uneti = ""
    if isinstance(request_body, dict):
        uneti = str(request_body.get("pin", "")).strip()
    elif isinstance(request_body, str):
        uneti = request_body.strip().strip('"')
    if uneti == PIN_BE:
        log("  🔓 PIN ispravan — kartica otključana")
        return {"status": "OK", "message": "PIN verifikovan"}
    log("  🔒 Pogrešan PIN")
    return {"status": "ERROR", "code": "E003", "message": "Pogrešan PIN"}

def resp_open_drawer():
    return {"status": "OK", "message": "Fioka otvorena"}

def resp_print_text():
    return {"status": "OK", "message": "Tekst odštampan"}

def resp_receipt(request_id):
    """Vraća sačuvani račun u tekst formatu (za štampu)."""
    # Prvo probaj da nađeš po requestId
    for f in sorted(RECEIPTS_DIR.glob("*.txt"), reverse=True):
        if request_id in f.stem:
            return {
                "contentType": "text/plain; charset=utf-8",
                "receiptText": f.read_text(encoding="utf-8"),
                "requestId": request_id,
            }
    return {
        "contentType": "text/plain; charset=utf-8",
        "receiptText": "Račun nije pronađen.",
        "requestId": request_id,
    }

def resp_receipt_html(request_id):
    """Vraća sačuvani račun u HTML formatu (za A4 štampu iz browsera)."""
    for f in sorted(RECEIPTS_DIR.glob("*.html"), reverse=True):
        if request_id in f.stem:
            return f.read_text(encoding="utf-8")
    return "<h1>Račun nije pronađen</h1>"

def _build_report(title, start_date=None, end_date=None):
    """Pravi izveštaj iz snimljenih računa."""
    _firma = ucitaj_firmu()
    invoices = []
    for f in sorted(INVOICES_DIR.glob("*.json")):
        try:
            data = json.loads(f.read_text(encoding="utf-8"))
            invoices.append(data)
        except Exception:
            continue

    total_payments_by_type = {}
    total_tax_by_label = {}
    per_tx_data = {}
    invoice_count = 0
    grand_total = 0.0
    grand_tax = 0.0

    for inv in invoices:
        invoice_count += 1
        # Plaćanja
        for p in inv.get("payments", []):
            ptype = p.get("type", "Other")
            amt = float(p.get("amount", 0))
            total_payments_by_type[ptype] = total_payments_by_type.get(ptype, 0.0) + amt
            grand_total += amt
        # Porezi
        for t in inv.get("taxItems", []):
            lbl = t.get("label", "")
            rate = float(t.get("rate", 0))
            amt = float(t.get("amount", 0))
            key = f"{lbl}_{rate}"
            if key not in total_tax_by_label:
                total_tax_by_label[key] = {"label": lbl, "rate": rate, "total": 0.0, "amount": 0.0}
            total_tax_by_label[key]["total"] += float(inv.get("totalAmount", 0))
            total_tax_by_label[key]["amount"] += amt
            grand_tax += amt
        # Po tipu transakcije
        tx = inv.get("transactionType", "NSX")
        if tx not in per_tx_data:
            per_tx_data[tx] = {"transactionTypeExt": tx, "invoiceCount": 0, "payments": {}, "taxItems": {}}
        per_tx_data[tx]["invoiceCount"] += 1
        for p in inv.get("payments", []):
            ptype = p.get("type", "Other")
            amt = float(p.get("amount", 0))
            per_tx_data[tx]["payments"][ptype] = per_tx_data[tx]["payments"].get(ptype, 0.0) + amt
        for t in inv.get("taxItems", []):
            lbl = t.get("label", "")
            rate = float(t.get("rate", 0))
            amt = float(t.get("amount", 0))
            key = f"{lbl}_{rate}"
            if key not in per_tx_data[tx]["taxItems"]:
                per_tx_data[tx]["taxItems"][key] = {"label": lbl, "rate": rate, "total": 0.0, "amount": 0.0}
            per_tx_data[tx]["taxItems"][key]["total"] += float(inv.get("totalAmount", 0))
            per_tx_data[tx]["taxItems"][key]["amount"] += amt

    # Formatiraj
    payments_list = [{"paymentType": k, "amount": v} for k, v in total_payments_by_type.items()]
    tax_list = list(total_tax_by_label.values())
    per_tx_list = []
    for tx, data in per_tx_data.items():
        tx_payments = [{"paymentType": k, "amount": v} for k, v in data["payments"].items()]
        tx_taxes = list(data["taxItems"].values())
        tx_total_pmts = sum(p["amount"] for p in tx_payments)
        tx_total_taxes = sum(t["amount"] for t in tx_taxes)
        per_tx_list.append({
            "transactionTypeExt": tx,
            "invoiceCount": data["invoiceCount"],
            "payments": tx_payments,
            "totalPayments": tx_total_pmts,
            "taxItems": tx_taxes,
            "totalTax": tx_total_taxes,
        })

    report_data = {
        "title": title,
        "number": 1,
        "dateTime": sada(),
        "tin": _firma["tin"],
        "businessName": _firma["name"],
        "locationName": _firma["locationName"],
        "address": _firma["address"],
        "district": _firma["district"],
        "uid": "550e8400-e29b-41d4-a716-000000000001",
        "startDate": start_date or datetime.now().strftime("%Y-%m-%d"),
        "endDate": end_date or datetime.now().strftime("%Y-%m-%d"),
        "total": {
            "invoiceCount": invoice_count,
            "payments": payments_list,
            "totalPayments": grand_total,
            "taxItems": tax_list,
            "totalTax": grand_tax,
        },
        "perTransactionType": per_tx_list,
    }
    return report_data

def resp_daily_report():
    today = datetime.now().strftime("%Y-%m-%d")
    locale = load_locale("latin")
    report_data = _build_report(locale.get("daily-report", "DNEVNI IZVEŠTAJ"), today, today)

    # Snimi izveštaj
    report_path = RECEIPTS_DIR / f"daily-report-{today}.json"
    with open(report_path, "w", encoding="utf-8") as f:
        json.dump(report_data, f, indent=2, ensure_ascii=False)

    # Generiši tekst izveštaj
    report_text = generate_report(report_data, "latin")
    text_path = RECEIPTS_DIR / f"daily-report-{today}.txt"
    text_path.write_text(report_text, encoding="utf-8")

    log(f"  📊 DNEVNI IZVEŠTAJ | računa: {report_data['total']['invoiceCount']} | ukupno: {report_data['total']['totalPayments']:.2f}")
    return report_data

def resp_periodic_report():
    today = datetime.now().strftime("%Y-%m-%d")
    locale = load_locale("latin")
    report_data = _build_report(locale.get("periodic-report", "PERIODIČNI IZVEŠTAJ"), "2026-01-01", today)

    report_path = RECEIPTS_DIR / f"periodic-report-{today}.json"
    with open(report_path, "w", encoding="utf-8") as f:
        json.dump(report_data, f, indent=2, ensure_ascii=False)

    report_text = generate_report(report_data, "latin")
    text_path = RECEIPTS_DIR / f"periodic-report-{today}.txt"
    text_path.write_text(report_text, encoding="utf-8")

    log(f"  📊 PERIODIČNI IZVEŠTAJ | računa: {report_data['total']['invoiceCount']} | ukupno: {report_data['total']['totalPayments']:.2f}")
    return report_data

def resp_notifications():
    return [{
        "id": "1",
        "type": "INFO",
        "message": "Sistem funkcioniše ispravno",
        "timestamp": sada(),
    }]

def resp_status_codes():
    return {
        "codes": [
            {"code": "S001", "description": "Uspešno potpisan račun"},
            {"code": "E001", "description": "Greška pri potpisivanju"},
            {"code": "E002", "description": "Kartica nije prisutna"},
            {"code": "E003", "description": "Pogrešan PIN"},
            {"code": "E004", "description": "Nema konekcije ka SUF serveru"},
        ],
    }

# Mapiranje handler-a
HANDLERS = {
    "attention":         resp_attention,
    "status":            resp_status,
    "environment":       resp_environment,
    "subject":           resp_subject,
    "invoice":           resp_invoice,
    "invoice_lookup":    resp_invoice_lookup,
    "verify_pin":        resp_verify_pin,
    "open_drawer":       resp_open_drawer,
    "print_text":        resp_print_text,
    "receipt":           resp_receipt,
    "receipt_text":      resp_receipt,
    "receipt_html":      resp_receipt_html,
    "daily_report":            resp_daily_report,
    "daily_report_text":       resp_daily_report,
    "periodic_report":         resp_periodic_report,
    "periodic_report_text":    resp_periodic_report,
    "notifications":     resp_notifications,
    "status_codes":      resp_status_codes,
}

# ── HTTP Handler ─────────────────────────────────────────────

class FiscalHandler(http.server.BaseHTTPRequestHandler):
    """Glavni handler za sve fiskalne endpoint-e."""

    def log_message(self, fmt, *args):
        """Override — koristi naš log umesto default stderr."""
        pass  # Logujemo ručno u _handle

    def do_GET(self):
        self._handle("GET")

    def do_POST(self):
        self._handle("POST")

    def do_OPTIONS(self):
        """CORS preflight."""
        self.send_response(200)
        self.send_header("Access-Control-Allow-Origin", "*")
        self.send_header("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
        self.send_header("Access-Control-Allow-Headers", "Content-Type, Accept-Language, RequestId")
        self.end_headers()

    def _handle(self, method):
        path = self.path.split("?")[0]
        hdrs = {
            "Access-Control-Allow-Origin": "*",
            "Access-Control-Allow-Methods": "GET,POST,OPTIONS",
            "Access-Control-Allow-Headers": "Content-Type, Accept-Language, RequestId",
        }
        status = 404
        body = None
        matched_route = None

        # Nađi rutu
        r_handler_name = None
        for r_method, r_pattern, r_handler_name in DEFAULT_ROUTES:
            if r_method != method:
                continue
            # Konvertuj :param u regex
            regex = re.sub(r":(\w+)", r"(?P<\1>[^/]+)", r_pattern)
            regex = f"^{regex}$"
            m = re.match(regex, path.strip("/"))
            if m:
                matched_route = r_pattern
                handler = HANDLERS.get(r_handler_name)
                if handler:
                    request_id = self.headers.get("RequestId", "unknown")
                    params = m.groupdict()

                    if r_handler_name == "invoice":
                        # Pročitaj telo zahteva
                        content_len = int(self.headers.get("Content-Length", 0))
                        request_body = None
                        if content_len > 0:
                            try:
                                raw = self.rfile.read(content_len)
                                request_body = json.loads(raw.decode("utf-8"))
                            except Exception:
                                request_body = None
                        body = handler(request_id, request_body)
                    elif r_handler_name == "verify_pin":
                        # Pročitaj telo (PIN) — JSON {"pin": "..."} ili goli string
                        content_len = int(self.headers.get("Content-Length", 0))
                        request_body = None
                        if content_len > 0:
                            try:
                                raw = self.rfile.read(content_len).decode("utf-8")
                                try:
                                    request_body = json.loads(raw)
                                except Exception:
                                    request_body = raw
                            except Exception:
                                request_body = None
                        body = handler(request_body)
                    elif r_handler_name == "invoice_lookup":
                        rid = params.get("requestId", "unknown")
                        result = handler(rid)
                        if result:
                            body = result
                        else:
                            status = 404
                            body = {"error": f"Račun {rid} nije pronađen"}
                    elif r_handler_name == "receipt":
                        body = handler(params.get("requestId", "unknown"))
                    elif r_handler_name == "receipt_text":
                        result = handler(params.get("requestId", "unknown"))
                        body = result.get("receiptText", "Račun nije pronađen.")
                    elif r_handler_name == "receipt_html":
                        body = handler(params.get("requestId", "unknown"))
                    else:
                        body = handler()
                    status = 200
                break

        # Log
        emoji = "✅" if status == 200 else "❌"
        client = self.client_address[0]
        log(f"  {emoji} {method} {path} → {matched_route or '404'} | {client}")

        # Pošalji odgovor
        if r_handler_name in ("daily_report_text", "periodic_report_text") and body:
            report_text = generate_report(body, "latin")
            resp_bytes = report_text.encode("utf-8")
            hdrs["Content-Type"] = "text/plain; charset=utf-8"
        elif r_handler_name == "receipt_html" and body:
            # HTML odgovor
            resp_bytes = body.encode("utf-8") if isinstance(body, str) else body
            hdrs["Content-Type"] = "text/html; charset=utf-8"
        elif isinstance(body, str) and r_handler_name == "receipt_text":
            # Tekst odgovor
            resp_bytes = body.encode("utf-8") if isinstance(body, str) else body
            hdrs["Content-Type"] = "text/plain; charset=utf-8"
        elif body is not None:
            resp_bytes = json.dumps(body, indent=2, ensure_ascii=False).encode("utf-8")
            hdrs["Content-Type"] = "application/json; charset=utf-8"
        else:
            resp_bytes = json.dumps({"error": "Not Found", "path": path}, ensure_ascii=False).encode("utf-8")
            hdrs["Content-Type"] = "application/json; charset=utf-8"

        self.send_response(status)
        for k, v in hdrs.items():
            self.send_header(k, v)
        self.send_header("Content-Length", str(len(resp_bytes)))
        self.end_headers()
        self.wfile.write(resp_bytes)

# ── Main ─────────────────────────────────────────────────────

def main():
    log("╔══════════════════════════════════════════════╗")
    log("║   🧾 myLPFR Mock Server — Fiskalni server   ║")
    log("║   http://{}:{}/                 ║".format(HOST, PORT))
    log("║   {} ruta | QR: AUTO | Snimanje: UKLJUČENO  ║".format(len(DEFAULT_ROUTES)))
    log("╚══════════════════════════════════════════════╝")
    log(f"  📁 Podaci: {DATA_DIR}")
    log(f"  🧾 Računi: {INVOICES_DIR}")
    log(f"  📱 QR PNG: {QR_DIR}")
    log(f"  📝 Log:    {LOG_FILE}")
    f = ucitaj_firmu()
    log(f"  💳 Kartica (BE) — baza: {NTECH_DB}")
    log(f"     Firma: {f['name']} | PIB: {f['tin']} | MB: {f['mb']}")
    log(f"     Adresa: {f['address']} | Test PIN: {PIN_BE}")
    log("  ▶  Server pokrenut. Ctrl+C za gašenje.")

    server = http.server.HTTPServer((HOST, PORT), FiscalHandler)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        log("  ⏹  Server zaustavljen.")
        server.server_close()

if __name__ == "__main__":
    main()
