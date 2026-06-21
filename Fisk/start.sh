#!/bin/bash
# ── start.sh — Pokreće fiskalni mock server ─────────────────
#   ./start.sh        — pokreće server u pozadini
#   ./start.sh -f     — pokreće server u prvom planu (vidiš logove)
#   ./start.sh status — proverava da li je server živ

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PID_FILE="$SCRIPT_DIR/data/server.pid"
PORT=8989

# Proveri da li server već radi
check_running() {
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        if kill -0 "$PID" 2>/dev/null; then
            return 0  # radi
        fi
    fi
    # Proveri i port
    if ss -tlnp 2>/dev/null | grep -q ":$PORT "; then
        return 0  # radi
    fi
    return 1  # ne radi
}

stop_server() {
    echo "⏹  Zaustavljam server..."
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        kill "$PID" 2>/dev/null && echo "   PID $PID ugašen"
        rm -f "$PID_FILE"
    fi
    fuser -k "${PORT}/tcp" 2>/dev/null
    echo "✅ Server zaustavljen"
}

case "${1:-}" in
    status)
        if check_running; then
            echo "🟢 Server je POKRENUT na http://localhost:$PORT"
            echo "   PID: $(cat "$PID_FILE" 2>/dev/null || echo '?')"
        else
            echo "🔴 Server NIJE pokrenut"
        fi
        ;;
    stop)
        stop_server
        ;;
    -f|--foreground)
        echo "🔧 Pokrećem server u PRVOM PLANU (Ctrl+C gasi)..."
        cd "$SCRIPT_DIR"
        python3 -u server.py
        ;;
    *)
        if check_running; then
            echo "⚠️  Server je VEĆ pokrenut na http://localhost:$PORT"
            echo "   Koristi './start.sh stop' da ga zaustaviš, ili './start.sh status'"
            exit 1
        fi
        echo "🔧 Pokrećem server u pozadini..."
        cd "$SCRIPT_DIR"
        mkdir -p data
        nohup python3 -u server.py > "$SCRIPT_DIR/data/server_stdout.log" 2>&1 &
        PID=$!
        echo "$PID" > "$PID_FILE"
        sleep 2
        if check_running; then
            echo "✅ Server pokrenut! PID: $PID"
            echo "   http://localhost:$PORT"
            echo ""
            echo "   ./start.sh stop   — zaustavi server"
            echo "   ./start.sh status — proveri status"
            echo "   ./start.sh -f     — pokreni u prvom planu (vidiš logove uživo)"
        else
            echo "❌ Server nije uspeo da se pokrene. Proveri: data/server_stdout.log"
            rm -f "$PID_FILE"
            exit 1
        fi
        ;;
esac
