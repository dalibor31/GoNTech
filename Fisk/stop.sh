#!/bin/bash
# ── stop.sh — Zaustavlja fiskalni mock server ───────────────

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PID_FILE="$SCRIPT_DIR/data/server.pid"
PORT=8989

echo "⏹  Zaustavljam fiskalni mock server..."

# Pokušaj preko PID fajla
if [ -f "$PID_FILE" ]; then
    PID=$(cat "$PID_FILE")
    if kill "$PID" 2>/dev/null; then
        echo "   PID $PID ugašen."
    fi
    rm -f "$PID_FILE"
fi

# Osiguraj da port nije zauzet
fuser -k "${PORT}/tcp" 2>/dev/null

sleep 0.5

# Proveri
if ss -tlnp 2>/dev/null | grep -q ":$PORT "; then
    echo "⚠️  Port $PORT je još uvek zauzet. Pokušaj: kill -9 \$(fuser ${PORT}/tcp)"
else
    echo "✅ Server zaustavljen. Port $PORT je slobodan."
fi
