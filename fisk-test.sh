#!/bin/bash
# ═══════════════════════════════════════════════════════════════
#  fisk-test.sh — Interaktivni fiskalni test server
# ═══════════════════════════════════════════════════════════════
#  Pokreće myLPFR Mock Server, pokazuje info,
#  i čeka da pritisneš 'q' da ga ugasiš.

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
FISK_DIR="$SCRIPT_DIR/Fisk"
PID_FILE="$FISK_DIR/data/server.pid"
PORT=8989
HOST="localhost"
BASE_URL="http://$HOST:$PORT"

# ── Boje za output ─────────────────────────────────────────
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# ── Cleanup na izlaz ───────────────────────────────────────
cleanup() {
    echo ""
    echo -e "${YELLOW}⏹  Gasim server...${NC}"

    # Preko PID fajla
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        kill "$PID" 2>/dev/null && echo -e "   PID ${BOLD}$PID${NC} ugašen"
        rm -f "$PID_FILE"
    fi

    # Pobrini se da port 8989 bude slobodan
    fuser -k "${PORT}/tcp" 2>/dev/null || true
    sleep 0.5

    if ss -tlnp 2>/dev/null | grep -q ":$PORT "; then
        echo -e "${RED}⚠️  Port $PORT je još uvek zauzet.${NC}"
        echo "   Ručno: kill -9 \$(fuser ${PORT}/tcp)"
    else
        echo -e "${GREEN}✅ Server zaustavljen. Port $PORT je slobodan.${NC}"
    fi
    exit 0
}

trap cleanup INT TERM

# ── Provera da li server već radi ──────────────────────────
check_running() {
    # Provera preko PID fajla
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        if kill -0 "$PID" 2>/dev/null; then
            return 0
        fi
    fi
    # Provera preko porta
    if ss -tlnp 2>/dev/null | grep -q ":$PORT " || \
       netstat -tlnp 2>/dev/null | grep -q ":$PORT "; then
        return 0
    fi
    return 1
}

# ── Dohvatanje PID-a servera ───────────────────────────────
get_pid() {
    if [ -f "$PID_FILE" ]; then
        cat "$PID_FILE"
    else
        ss -tlnp 2>/dev/null | grep ":$PORT " | sed -E 's/.*pid=([0-9]+).*/\1/' | head -1
    fi
}

# ═══════════════════════════════════════════════════════════════
#  MAIN
# ═══════════════════════════════════════════════════════════════

echo ""
echo -e "${BOLD}╔══════════════════════════════════════════════════╗${NC}"
echo -e "${BOLD}║   🧾  myLPFR Fiskalni Test Server               ║${NC}"
echo -e "${BOLD}╚══════════════════════════════════════════════════╝${NC}"
echo ""

if check_running; then
    PID=$(get_pid)
    echo -e "${GREEN}🟢 Server je VEĆ pokrenut${NC}"
    echo -e "   URL: ${CYAN}$BASE_URL${NC}"
    echo -e "   PID: ${BOLD}$PID${NC}"
else
    echo -e "${YELLOW}🔧 Pokrećem fiskalni server...${NC}"

    # Osiguraj data folder
    mkdir -p "$FISK_DIR/data"

    # Pokreni server u pozadini
    cd "$FISK_DIR"
    nohup python3 -u server.py > "$FISK_DIR/data/server_stdout.log" 2>&1 &
    PID=$!
    echo "$PID" > "$PID_FILE"
    cd "$SCRIPT_DIR"

    # Sačekaj da server bude spreman
    echo -n "   Čekam server"
    for i in $(seq 1 20); do
        sleep 0.3
        echo -n "."
        if check_running; then
            echo ""
            break
        fi
    done

    if check_running; then
        echo ""
        echo -e "${GREEN}✅ Server pokrenut!${NC}"
    else
        echo ""
        echo -e "${RED}❌ Server nije uspeo da se pokrene!${NC}"
        echo "   Proveri log: tail -f $FISK_DIR/data/server_stdout.log"
        rm -f "$PID_FILE"
        exit 1
    fi
fi

# ── Prikaži info o serveru ─────────────────────────────────
echo ""
echo -e "${BOLD}── Podaci o serveru ──────────────────────────────${NC}"
echo -e "  🌐 URL:         ${CYAN}$BASE_URL${NC}"
echo -e "  📦 PID:         ${BOLD}$(get_pid)${NC}"
echo -e "  📁 Data dir:    ${FISK_DIR}/data"
echo -e "  🧾 Računi:      ${FISK_DIR}/data/invoices"
echo -e "  📱 QR kodovi:   ${FISK_DIR}/data/qr"
echo -e "  📝 Log servera: ${FISK_DIR}/data/server.log"
echo -e "  📤 Stdout log:  ${FISK_DIR}/data/server_stdout.log"
echo ""

# Prikaži trenutni brojač ako postoji
COUNTER_FILE="$FISK_DIR/data/counter.txt"
if [ -f "$COUNTER_FILE" ]; then
    echo -e "  🔢 Sledeći račun: ${BOLD}$(cat "$COUNTER_FILE")${NC}"
else
    echo -e "  🔢 Sledeći račun: ${BOLD}000001${NC}"
fi

# Prikaži poslednjih 5 log linija servera
if [ -f "$FISK_DIR/data/server.log" ] && [ -s "$FISK_DIR/data/server.log" ]; then
    echo ""
    echo -e "${BOLD}── Poslednje log linije ─────────────────────────${NC}"
    tail -5 "$FISK_DIR/data/server.log" | while read -r line; do
        echo -e "  ${CYAN}$line${NC}"
    done
fi

echo ""
echo -e "${BOLD}──────────────────────────────────────────────────${NC}"
echo -e "  ${GREEN}Server je spreman za testiranje.${NC}"
echo -e "  Pritisni ${BOLD}q${NC} + Enter da zaustaviš server."
echo -e "${BOLD}──────────────────────────────────────────────────${NC}"
echo ""

# ── Čekaj 'q' ─────────────────────────────────────────────
while true; do
    read -r -p "  ⌨  Unesi 'q' za gašenje: " input
    if [ "$input" = "q" ] || [ "$input" = "Q" ]; then
        cleanup
    fi
done
