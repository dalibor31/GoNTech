#!/usr/bin/env python3
"""
Generisanje fiskalnog računa za A4 štampač.
Prati izgled definisan u LPFR VM template-u i locale fajlovima.
"""

from pathlib import Path
from datetime import datetime
import io

from reportlab.lib.pagesizes import A4
from reportlab.pdfgen import canvas
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont

DATA_DIR = Path(__file__).parent / "data"

_FONT_NAME = "Courier"
_FONT_PATH = Path("/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf")
if _FONT_PATH.exists():
    try:
        pdfmetrics.registerFont(TTFont("DejaVuSansMono", str(_FONT_PATH)))
        _FONT_NAME = "DejaVuSansMono"
    except Exception:
        pass

def load_locale(lang="latin"):
    """Učitava lokalizacioni fajl u dict."""
    filename = f"locale_{lang}.properties"
    path = DATA_DIR / filename
    if not path.exists():
        return {}
    locale = {}
    with open(path, encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line or line.startswith("#") or "=" not in line:
                continue
            key, _, value = line.partition("=")
            locale[key.strip()] = value.strip()
    return locale

def _sr(n, decimals=2):
    """Srpski format: tačka za hiljade, zarez za decimale — 2.967,17"""
    s = f"{n:,.{decimals}f}"
    return s.replace(",", "\x00").replace(".", ",").replace("\x00", ".")

def price(n):
    """Formatira cenu: ###.###,00"""
    return _sr(n, 2)

def qty(n):
    """Formatira količinu: ###.###,000"""
    return _sr(n, 3)

def amount(n):
    """Formatira iznos: ###.###,00"""
    return _sr(n, 2)

def number(n):
    """Formatira broj: ###.###"""
    return _sr(n, 0)

def dt(iso_string):
    """Konvertuje ISO datetime u format: dd.MM.yyyy. HH:mm:ss"""
    try:
        d = datetime.fromisoformat(iso_string.replace("+02:00", "").replace("Z", ""))
        return d.strftime("%d.%m.%Y. %H:%M:%S")
    except Exception:
        return iso_string

def center(text, width=48):
    """Centrira tekst unutar širine."""
    return text.center(width)

def layout(left, right, width=48):
    """Formatira dve kolone: levo poravnato levo, desno poravnato desno."""
    left_str = str(left) if left else ""
    right_str = str(right) if right else ""
    space = width - len(left_str) - len(right_str)
    if space < 1:
        space = 1
    return left_str + " " * space + right_str

def wrap(text, width=48):
    """Prelamanje teksta (vraća listu linija)."""
    if not text:
        return []
    words = text.split()
    lines = []
    current = ""
    for word in words:
        if len(current) + len(word) + 1 <= width:
            current = (current + " " + word).strip()
        else:
            if current:
                lines.append(current)
            current = word
    if current:
        lines.append(current)
    return lines if lines else [text]

def separator(char="-", width=48):
    """Linija separatora."""
    return char * width

def title(text, char="=", width=48):
    """Naslov okružen linijama."""
    return separator(char, width) + "\n" + center(text, width) + "\n" + separator(char, width)

# ── Transakcije prevod ──────────────────────────────────────
TRANSACTION_TYPES_CYR = {
    "NSX": "ПРОМЕТ - ПРОДАЈА", "NRX": "ПРОМЕТ - РЕФУНДАЦИЈА",
    "CSX": "КОПИЈА - ПРОДАЈА", "CRX": "КОПИЈА - РЕФУНДАЦИЈА",
    "ASX": "АВАНС - ПРОДАЈА", "ARX": "АВАНС - РЕФУНДАЦИЈА",
    "PSX": "ПРЕДРАЧУН - ПРОДАЈА", "PRX": "ПРЕДРАЧУН - РЕФУНДАЦИЈА",
    "TSX": "ОБУКА - ПРОДАЈА", "TRX": "ОБУКА - РЕФУНДАЦИЈА",
}
TRANSACTION_TYPES_LAT = {
    "NSX": "PROMET - PRODAJA", "NRX": "PROMET - REFUNDACIJA",
    "CSX": "KOPIJA - PRODAJA", "CRX": "KOPIJA - REFUNDACIJA",
    "ASX": "AVANS - PRODAJA", "ARX": "AVANS - REFUNDACIJA",
    "PSX": "PREDRAČUN - PRODAJA", "PRX": "PREDRAČUN - REFUNDACIJA",
    "TSX": "OBUKA - PRODAJA", "TRX": "OBUKA - REFUNDACIJA",
}
# NTech šalje invoiceType+transactionType kao reči (Normal/Sale) umesto koda (NSX)
_INV_TX_TO_CODE = {
    ("Normal",   "Sale"):   "NSX", ("Normal",   "Refund"): "NRX",
    ("Advance",  "Sale"):   "ASX", ("Advance",  "Refund"): "ARX",
    ("Copy",     "Sale"):   "CSX", ("Copy",     "Refund"): "CRX",
    ("Training", "Sale"):   "TSX", ("Training", "Refund"): "TRX",
    ("Proforma", "Sale"):   "PSX", ("Proforma", "Refund"): "PRX",
}

def _tx_code(inv):
    """Vraća 3-slovni kod transakcije (NSX, NRX...) iz full_data rečnika."""
    it = inv.get("invoiceType", "Normal")
    tt = inv.get("transactionType", "Sale")
    return _INV_TX_TO_CODE.get((it, tt), tt)

# ── Glavna funkcija ─────────────────────────────────────────

def generate_receipt(invoice_data, lang="latin"):
    """
    Generiše fiskalni račun spreman za A4 štampu.
    Prati agent-invoice.vm template 1:1.
    """
    m = load_locale(lang)
    if not m:
        m = load_locale("latin")

    tx_types = TRANSACTION_TYPES_LAT if lang == "latin" else TRANSACTION_TYPES_CYR
    W = 48
    inv = invoice_data
    tx_label = tx_types.get(_tx_code(inv), _tx_code(inv))
    lines = []

    # ── PREAMBLE ──
    preamble = inv.get("preamble", "")
    if preamble:
        for pl in wrap(preamble, W):
            lines.append(pl)
        lines.append("")

    # ── NASLOV ──
    is_fiscal = inv.get("isFiscal", True)
    if is_fiscal:
        lines.append(title(m.get("fiscal-invoice", "FISKALNI RAČUN"), "=", W))
    else:
        lines.append(title(m.get("non-fiscal-invoice", "OVO NIJE FISKALNI RAČUN"), "=", W))

    # ── ZAGLAVLJE ──
    for field in ["tin", "company", "store", "address", "district"]:
        val = inv.get(field, "")
        if val:
            lines.append(center(str(val), W))
    lines.append(separator("-", W))

    # Buyer info
    if inv.get("buyerId"):
        lines.append(layout(m.get("buyer-id", "ID kupca"), inv["buyerId"], W))
    if inv.get("buyerCostCenterId"):
        lines.append(layout(m.get("buyer-cost-center", "Opcija kupca"), inv["buyerCostCenterId"], W))
    if inv.get("cashier"):
        lines.append(layout(m.get("cashier-id", "Kasir"), inv["cashier"], W))
    if inv.get("posNumber"):
        lines.append(layout(m.get("pos-invoice-number", "ESIR broj"), inv["posNumber"], W))
    if inv.get("posDateTime"):
        lines.append(layout(m.get("pos-time", "ESIR vreme"), dt(inv["posDateTime"]), W))

    # Referentni dokument (storno)
    if inv.get("referentDocumentNumber"):
        lines.append(layout(m.get("ref-doc-number", "Ref. broj"), inv["referentDocumentNumber"], W))
    if inv.get("referentDocumentDT"):
        lines.append(layout(m.get("ref-doc-dt", "Ref. vreme"), dt(inv["referentDocumentDT"]), W))

    # ── TIP TRANSAKCIJE ──
    lines.append(title(tx_label, "-", W))

    # ── ARTIKLI ──
    lines.append(center(m.get("items", "Artikli"), W))
    lines.append(separator("=", W))

    # Zaglavlje tabele (prati agent-invoice.vm layout)
    lbl_name = m.get("item-name", "Naziv")
    lbl_price = m.get("item-price", "Cena")
    lbl_qty = m.get("item-qty", "Kol.")
    lbl_amount = m.get("item-amount", "Ukupno")
    header = lbl_name + lbl_price.rjust(10) + lbl_qty.rjust(11)
    lines.append(layout(header, lbl_amount, W))

    for item in inv.get("items", []):
        name = item.get("name", "")
        gtin = item.get("gtin", "")
        labels = item.get("labels", [])
        label_str = " ".join(labels) if labels else ""
        if gtin:
            name_line = f"{gtin} {name} {label_str}".strip()
        else:
            name_line = f"{name} {label_str}".strip()
        for wl in wrap(name_line, W):
            lines.append(wl)

        ip = float(item.get("unitPrice") or item.get("price", 0))
        iq = float(item.get("quantity") or item.get("qty", 0))
        ia = float(item.get("amount") or (ip * iq))
        sign = "-" if inv.get("transactionType") == "Refund" else ""
        row = price(ip).rjust(14) + qty(iq).rjust(9)
        lines.append(layout(row, sign + amount(ia), W))

    lines.append(separator("-", W))

    # ── UKUPNO ──
    if inv.get("transactionType") == "Refund":
        lines.append(layout(m.get("total-refund", "Ukupna refundacija"), amount(inv.get("totalAmount", 0)), W))
    else:
        lines.append(layout(m.get("to-pay", "Za uplatu"), amount(inv.get("totalAmount", 0)), W))

    # ── PLAĆANJA ──
    advance = float(inv.get("advance", 0))
    advance_tax = float(inv.get("advanceTax", 0))
    is_covered_by_advance = inv.get("coveredByAdvance", False)

    if advance:
        lines.append(layout(m.get("paid-in-advance", "Uplaćeno avansom"), amount(advance), W))
    if advance_tax:
        lines.append(layout(m.get("advance-tax", "PDV na avans"), amount(advance_tax), W))

    if not is_covered_by_advance:
        for p in inv.get("payments", []):
            pt = p.get("paymentType", p.get("type", ""))
            ptype = m.get(pt, pt or "Drugo")
            lines.append(layout(ptype, amount(float(p.get("amount", 0))), W))
        if inv.get("invoiceType") == "Proforma":
            lines.append(layout(m.get("refund", "Povraćaj"), amount(0), W))
        else:
            lines.append(layout(m.get("refund", "Povraćaj"), amount(float(inv.get("refund", 0))), W))

    if advance:
        lines.append(layout(m.get("remaining", "Preostalo"), amount(float(inv.get("remaining", 0))), W))

    lines.append(separator("=", W))

    # ── NEFISKALNI RAČUN ──
    if not is_fiscal:
        lines.append(center(m.get("non-fiscal-invoice", "OVO NIJE FISKALNI RAČUN"), W))
        lines.append(separator("-", W))

    # ── POREZI ──
    tax_hdr = (m.get("tax-label", "Oznaka") +
               m.get("tax-name", "Ime").rjust(8) +
               m.get("tax-rate", "Stopa").rjust(8))
    lines.append(layout(tax_hdr, m.get("tax-amount", "Porez"), W))
    for tax in inv.get("taxItems", []):
        row = (str(tax.get("label", "")) +
               str(tax.get("name", "")).rjust(13) +
               number(float(tax.get("rate", 0))).rjust(7) + "%")
        lines.append(layout(row, amount(float(tax.get("amount", 0))), W))
    lines.append(separator("-", W))
    lines.append(layout(m.get("total-tax", "Ukupan porez"), amount(float(inv.get("totalTax", 0))), W))
    lines.append(separator("=", W))

    # ── PFR VREDNOSTI ──
    lines.append(layout(m.get("sdc-time", "PFR vreme"), dt(inv.get("sdcDateTime", "")), W))
    lines.append(layout(m.get("sdc-invoice-number", "PFR broj računa"), str(inv.get("invoiceNumber", "")), W))
    lines.append(layout(m.get("sdc-invoice-counter", "Brojač računa"), str(inv.get("invoiceCounter", "")), W))
    lines.append(separator("=", W))

    # ── QR KOD ──
    lines.append("{{{{QR-KOD}}}}")
    lines.append("")

    # ── POTPIS KUPCA (Copy + Refund) ──
    if inv.get("invoiceType") == "Copy" and inv.get("transactionType") == "Refund":
        lines.append("")
        lines.append(f"{m.get('customer-signature', 'Potpis kupca')}: ______________________")
        lines.append("")

    # ── KRAJ ──
    if is_fiscal:
        lines.append(title(m.get("end-of-fiscal-invoice", "KRAJ FISKALNOG RAČUNA"), "=", W))
    else:
        lines.append(title(m.get("non-fiscal-invoice", "OVO NIJE FISKALNI RAČUN"), "=", W))

    # ── PORUKA ──
    msg = inv.get("message", "")
    if msg:
        lines.append("")
        for ml in wrap(msg, W):
            lines.append(ml)

    return "\n".join(lines)


def generate_receipt_html(invoice_data, lang="latin"):
    """
    Generiše HTML verziju fiskalnog računa — za A4 štampu iz browsera.
    Prati agent-invoice.vm template 1:1.
    """
    m = load_locale(lang)
    if not m:
        m = load_locale("latin")

    tx_types = TRANSACTION_TYPES_LAT if lang == "latin" else TRANSACTION_TYPES_CYR
    inv = invoice_data

    tx_label = tx_types.get(_tx_code(inv), _tx_code(inv))
    is_fiscal = inv.get("isFiscal", True)
    is_refund = inv.get("transactionType") == "Refund"
    inv_type = inv.get("invoiceType", "Normal")
    sign = "-" if is_refund else ""

    # Preamble
    preamble_html = ""
    preamble = inv.get("preamble", "")
    if preamble:
        preamble_html = f'<div class="preamble">{preamble}</div><div class="sep"></div>'

    # Items
    items_rows = ""
    for item in inv.get("items", []):
        gtin = f' <small>(GTIN: {item["gtin"]})</small>' if item.get("gtin") else ""
        labels = " ".join(item.get("labels", []))
        ip = float(item.get("unitPrice") or item.get("price", 0))
        iq = float(item.get("quantity") or item.get("qty", 0))
        ia = float(item.get("amount") or (ip * iq))
        items_rows += f"""
        <tr>
            <td class="l">{item.get('name', '')}{gtin} {labels}</td>
            <td class="r">{price(ip)}</td>
            <td class="r">{qty(iq)}</td>
            <td class="r">{sign}{amount(ia)}</td>
        </tr>"""

    # Payments
    payments_rows = ""
    advance = float(inv.get("advance", 0))
    advance_tax = float(inv.get("advanceTax", 0))
    covered = inv.get("coveredByAdvance", False)

    if advance:
        payments_rows += f'<tr><td class="l">{m.get("paid-in-advance", "Uplaćeno avansom")}</td><td class="r">{amount(advance)}</td></tr>'
    if advance_tax:
        payments_rows += f'<tr><td class="l">{m.get("advance-tax", "PDV na avans")}</td><td class="r">{amount(advance_tax)}</td></tr>'
    if not covered:
        for p in inv.get("payments", []):
            pt = p.get("paymentType", p.get("type", ""))
            ptype = m.get(pt, pt or "Drugo")
            payments_rows += f'<tr><td class="l">{ptype}</td><td class="r">{amount(float(p.get("amount", 0)))}</td></tr>'
        if inv_type == "Proforma":
            payments_rows += f'<tr><td class="l">{m.get("refund", "Povraćaj")}</td><td class="r">{amount(0)}</td></tr>'
        else:
            payments_rows += f'<tr><td class="l">{m.get("refund", "Povraćaj")}</td><td class="r">{amount(float(inv.get("refund", 0)))}</td></tr>'
    if advance:
        payments_rows += f'<tr><td class="l">{m.get("remaining", "Preostalo")}</td><td class="r">{amount(float(inv.get("remaining", 0)))}</td></tr>'

    # Non-fiscal notice
    non_fiscal_html = ""
    if not is_fiscal:
        non_fiscal_html = f'<div class="c"><strong>{m.get("non-fiscal-invoice", "OVO NIJE FISKALNI RAČUN")}</strong></div><div class="sep"></div>'

    # Tax
    tax_rows = ""
    for tax in inv.get("taxItems", []):
        tax_rows += f"""
        <tr>
            <td class="l">{tax.get('label', '')}</td>
            <td class="l">{tax.get('name', '')}</td>
            <td class="r">{number(float(tax.get('rate', 0)))}%</td>
            <td class="r">{amount(float(tax.get('amount', 0)))}</td>
        </tr>"""

    # Customer signature (Copy + Refund)
    sig_html = ""
    if inv_type == "Copy" and inv.get("transactionType") == "Refund":
        sig_html = f'<div style="margin-top:5mm;">{m.get("customer-signature", "Potpis kupca")}: ______________________</div>'

    # Message
    msg_html = ""
    msg = inv.get("message", "")
    if msg:
        msg_html = f'<div class="sep"></div><div>{msg}</div>'

    qr_src = f"data:image/png;base64,{inv.get('qrCode', '')}"
    title_text = m.get("fiscal-invoice", "FISKALNI RAČUN") if is_fiscal else m.get("non-fiscal-invoice", "OVO NIJE FISKALNI RAČUN")
    end_text = m.get("end-of-fiscal-invoice", "KRAJ FISKALNOG RAČUNA") if is_fiscal else m.get("non-fiscal-invoice", "")
    
    # ── Buyer info (dvokolonski: levo=header, desno=buyer) ──
    buyer_col = ""
    if inv.get("buyerId"):
        buyer_col += f'<div class="buyer-line"><span class="buyer-lbl">{m.get("buyer-id", "ID kupca")}</span><span>{inv["buyerId"]}</span></div>'
    if inv.get("buyerCostCenterId"):
        buyer_col += f'<div class="buyer-line"><span class="buyer-lbl">{m.get("buyer-cost-center", "Opcija kupca")}</span><span>{inv["buyerCostCenterId"]}</span></div>'

    ref_doc = ""
    if inv.get("referentDocumentNumber"):
        ref_doc += f'<div class="ref-line"><span>{m.get("ref-doc-number", "Ref. broj")}: {inv["referentDocumentNumber"]}</span></div>'
    if inv.get("referentDocumentDT"):
        ref_doc += f'<div class="ref-line"><span>{m.get("ref-doc-dt", "Ref. vreme")}: {dt(inv["referentDocumentDT"])}</span></div>'

    html = f"""<!DOCTYPE html>
<html lang="sr">
<head>
<meta charset="utf-8">
<title>{title_text} | {inv.get('invoiceNumber', '')}</title>
<style>
  @page {{ size: A4; margin: 8mm 11mm; }}
  body {{ font-family: 'Consolas', 'Lucida Console', 'IBM Plex Mono', 'Roboto Condensed', monospace; font-size: 7pt; width: 188mm; margin: 0 auto; color: #000; }}
  .c {{ text-align: center; }}
  .r {{ text-align: right; }}
  .l {{ text-align: left; }}
  .sep {{ border-top: 1px solid #000; margin: 1mm 0; }}
  .sep-double {{ border-top: 3px double #000; margin: 1mm 0; }}
  table {{ width: 100%; border-collapse: collapse; }}
  td {{ padding: 0.5px 1px; vertical-align: top; font-size: 7pt; }}
  th {{ padding: 0.5px 1px; font-size: 7pt; font-weight: normal; }}
  .hdr {{ font-size: 11pt; font-weight: bold; margin: 0 0 0.5mm 0; }}
  .hdr-sub {{ font-size: 11pt; font-weight: bold; }}
  .title {{ font-weight: bold; font-size: 12pt; margin: 1.5mm 0; }}
  .qr {{ text-align: center; margin: 2mm 0; }}
  .qr img {{ width: 60mm; height: 60mm; }}
  .preamble {{ font-style: italic; margin: 1mm 0; font-size: 7pt; }}
  .row {{ display: flex; justify-content: space-between; }}
  .col-left {{ width: 72%; }}
  .col-right {{ width: 25%; text-align: right; }}
  .buyer-line {{ font-size: 7pt; }}
  .buyer-lbl {{ margin-right: 3mm; }}
  .ref-line {{ font-size: 7pt; }}
  .inv-total {{ font-size: 8pt; font-weight: bold; }}
  .end-title {{ font-size: 9pt; font-weight: bold; }}
  @media print {{ body {{ -webkit-print-color-adjust: exact; }} }}
</style>
</head>
<body>
{preamble_html}
<div class="c title">{title_text}</div>
<div class="sep-double"></div>

<div class="c hdr">{inv.get('company', '')}</div>
<div class="c hdr-sub">{inv.get('tin', '')}</div>
<div class="c hdr-sub">{inv.get('store', '')}</div>
<div class="c hdr-sub">{inv.get('address', '')}</div>
<div class="c hdr-sub">{inv.get('district', '')}</div>
<div class="sep"></div>

<div class="row">
  <div class="col-left">
    <div class="ref-line">{m.get('cashier-id', 'Kasir')}: {inv.get('cashier', '')}</div>
    <div class="ref-line">{m.get('pos-invoice-number', 'ESIR broj')}: {inv.get('posNumber', '')}</div>
    <div class="ref-line">{m.get('pos-time', 'ESIR vreme')}: {dt(inv.get('posDateTime', ''))}</div>
  </div>
  <div class="col-right">
    {buyer_col}
  </div>
</div>
{ref_doc}
<div class="sep"></div>

<div class="c"><strong>{tx_label}</strong></div>
<div class="sep"></div>

<table>
<tr><th class="l">{m.get('item-name', 'Naziv')}</th><th class="r">{m.get('item-price', 'Cena')}</th><th class="r">{m.get('item-qty', 'Kol.')}</th><th class="r">{m.get('item-amount', 'Ukupno')}</th></tr>
{items_rows}
</table>
<div class="sep"></div>

<table>
<tr class="inv-total"><td class="l">{m.get('to-pay', 'Za uplatu') if not is_refund else m.get('total-refund', 'Ukupna refundacija')}</td><td class="r">{amount(float(inv.get('totalAmount', 0)))}</td></tr>
{payments_rows}
</table>
<div class="sep-double"></div>

{non_fiscal_html}

<table>
<tr><th class="l">{m.get('tax-label', 'Oznaka')}</th><th class="l">{m.get('tax-name', 'Naziv')}</th><th class="r">{m.get('tax-rate', 'Stopa')}</th><th class="r">{m.get('tax-amount', 'Porez')}</th></tr>
{tax_rows}
</table>
<div class="sep"></div>
<table>
<tr class="inv-total"><td class="l">{m.get('total-tax', 'Ukupan porez')}</td><td class="r">{amount(float(inv.get('totalTax', 0)))}</td></tr>
</table>
<div class="sep-double"></div>

<table>
<tr><td class="l">{m.get('sdc-time', 'PFR vreme')}</td><td class="r">{dt(inv.get('sdcDateTime', ''))}</td></tr>
<tr><td class="l">{m.get('sdc-invoice-number', 'PFR broj računa')}</td><td class="r">{inv.get('invoiceNumber', '')}</td></tr>
<tr><td class="l">{m.get('sdc-invoice-counter', 'Brojač računa')}</td><td class="r">{inv.get('invoiceCounter', '')}</td></tr>
</table>
<div class="sep-double"></div>

<div class="qr"><img src="{qr_src}" alt="QR kod za verifikaciju"></div>

{sig_html}

<div class="c end-title">{end_text}</div>
{msg_html}
</body>
</html>"""
    return html


# ═══════════════════════════════════════════════════════════════
#  DNEVNI / PERIODIČNI IZVEŠTAJI  (standard-report.vm)
# ═══════════════════════════════════════════════════════════════

def generate_report(report_data, lang="latin"):
    """
    Generiše dnevni ili periodični izveštaj.
    Prati standard-report.vm template 1:1.

    report_data = {
        "title": "DNEVNI IZVEŠTAJ",
        "number": 1,
        "dateTime": "2026-06-21T16:00:00.000+02:00",
        "tin": "123456789",
        "businessName": "Test Company DOO",
        "locationName": "Test Location",
        "address": "Test Address 1, Beograd",
        "district": "Savski Venac",
        "uid": "550e8400...",
        "startDate": "2026-06-21",
        "endDate": "2026-06-21",
        "total": {
            "invoiceCount": 42,
            "payments": [
                {"paymentType": "Cash", "amount": 25000.00},
                {"paymentType": "Card", "amount": 15000.00},
            ],
            "totalPayments": 40000.00,
            "taxItems": [
                {"label": "S", "rate": 20.0, "total": 250000.00, "amount": 50000.00},
                {"label": "P", "rate": 10.0, "total": 100000.00, "amount": 10000.00},
            ],
            "totalTax": 60000.00,
        },
        "perTransactionType": [
            {
                "transactionTypeExt": "NSX",
                "invoiceCount": 30,
                "payments": [...],
                "totalPayments": 30000.00,
                "taxItems": [...],
                "totalTax": 45000.00,
            }
        ],
    }
    """
    m = load_locale(lang)
    if not m:
        m = load_locale("latin")

    W = 48
    rep = report_data
    lines = []

    lines.append(separator("=", W))

    # Zaglavlje
    for field in ["tin", "businessName", "locationName", "address", "district"]:
        val = rep.get(field, "")
        if val:
            lines.append(center(str(val), W))
    lines.append(separator("-", W))

    # Naslov
    title_text = rep.get("title", "IZVEŠTAJ")
    lines.append(center(title_text, W))

    # Period
    start = rep.get("startDate", "")
    end = rep.get("endDate", "")
    if start and end:
        period_text = m.get("period", "PERIOD") + f": {start} - {end}"
        lines.append(center(period_text, W))
    lines.append(separator("-", W))

    # Broj izveštaja, JID, vreme
    if rep.get("number"):
        lines.append(layout(m.get("report-number", "Broj izveštaja"), str(rep["number"]), W))
    lines.append(layout(m.get("uid", "JID"), rep.get("uid", ""), W))
    lines.append(layout(m.get("sdc-time", "PFR vreme"), dt(rep.get("dateTime", "")), W))
    lines.append(separator("-", W))
    lines.append(center(m.get("summary", "UKUPAN PROMET"), W))
    lines.append(separator("-", W))

    total = rep.get("total", {})

    # Broj računa
    lines.append(layout(m.get("invoice-count", "Broj računa"), str(total.get("invoiceCount", 0)), W))

    # Plaćanja
    lines.append(title(m.get("payment", "Uplaćeno"), "=", W))
    for p in total.get("payments", []):
        ptype = m.get(p.get("paymentType", ""), p.get("paymentType", "Drugo"))
        lines.append(layout(ptype, amount(float(p.get("amount", 0))), W))
    lines.append(separator("-", W))
    lines.append(layout(m.get("total-payment", "Ukupno uplaćeno"), amount(float(total.get("totalPayments", 0))), W))

    # Porezi
    lines.append(title(m.get("tax", "Porez"), "=", W))
    tax_hdr = m.get("tax-rate", "Stopa") + m.get("item-amount", "Osnovica").rjust(20)
    lines.append(layout(tax_hdr, m.get("tax-amount", "Porez"), W))
    lines.append(separator("-", W))
    for t in total.get("taxItems", []):
        row = str(t.get("label", "")) + number(float(t.get("rate", 0))).rjust(7) + amount(float(t.get("total", 0))).rjust(17)
        lines.append(layout(row, amount(float(t.get("amount", 0))), W))
    lines.append(separator("-", W))
    lines.append(layout(m.get("total-tax", "Ukupan porez"), amount(float(total.get("totalTax", 0))), W))

    # Po tipu transakcije
    per_tx = rep.get("perTransactionType", [])
    if per_tx:
        lines.append(separator("=", W))
        lines.append(center(m.get("per-transaction-type", "PROMET PO VRSTI"), W))

        for summary in per_tx:
            lines.append(separator("=", W))
            tx_code = summary.get("transactionTypeExt", "")
            tx_label_full = TRANSACTION_TYPES_LAT.get(tx_code, "") if lang == "latin" else TRANSACTION_TYPES_CYR.get(tx_code, "")
            if tx_label_full:
                lines.append(center(tx_label_full, W))
            lines.append(separator("-", W))
            lines.append(layout(m.get("invoice-count", "Broj računa"), str(summary.get("invoiceCount", 0)), W))
            lines.append(title(m.get("payment", "Uplaćeno"), "=", W))
            for p in summary.get("payments", []):
                ptype = m.get(p.get("paymentType", ""), p.get("paymentType", "Drugo"))
                lines.append(layout(ptype, amount(float(p.get("amount", 0))), W))
            lines.append(separator("-", W))
            lbl = m.get("total-refund", "Refundacija") if "Refund" in tx_code else m.get("total-payment", "Ukupno")
            lines.append(layout(lbl, amount(float(summary.get("totalPayments", 0))), W))
            lines.append(title(m.get("tax", "Porez"), "=", W))
            lines.append(layout(tax_hdr, m.get("tax-amount", "Porez"), W))
            lines.append(separator("-", W))
            for t in summary.get("taxItems", []):
                row = str(t.get("label", "")) + number(float(t.get("rate", 0))).rjust(7) + amount(float(t.get("total", 0))).rjust(17)
                lines.append(layout(row, amount(float(t.get("amount", 0))), W))
            lines.append(separator("-", W))
            lines.append(layout(m.get("total-tax", "Ukupan porez"), amount(float(summary.get("totalTax", 0))), W))

    lines.append(separator("=", W))
    return "\n".join(lines)


def render_report_pdf(report_text):
    """Renderuje monospace tekst izveštaja (48 kolona) u pravi PDF (A4, Courier).
    Vraća bajtove gotovog PDF fajla."""
    buf = io.BytesIO()
    c = canvas.Canvas(buf, pagesize=A4)
    width, height = A4

    font_name = _FONT_NAME
    font_size = 10
    line_height = font_size * 1.15
    margin_left = 20 * (72 / 25.4)   # 20mm u tačkama
    margin_top = 15 * (72 / 25.4)    # 15mm u tačkama

    c.setFont(font_name, font_size)
    y = height - margin_top
    for line in report_text.split("\n"):
        if y < margin_top:
            c.showPage()
            c.setFont(font_name, font_size)
            y = height - margin_top
        c.drawString(margin_left, y, line)
        y -= line_height
    c.showPage()
    c.save()
    return buf.getvalue()
