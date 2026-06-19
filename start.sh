#!/bin/bash
set -e

GITEA_IMAGE="git.vm-net.in.rs/dasko/ntech"
GITHUB_IMAGE="ghcr.io/dalibor31/ntech"
VER_FAJL="VERSION"

clear
echo "╔══════════════════════════════════════╗"
echo "║        NTech — Build alat            ║"
echo "╚══════════════════════════════════════╝"
echo ""

# 1. Verzija
VERZIJA_DEFAULT=$(cat "$VER_FAJL" 2>/dev/null || echo "0.0.0")
read -p "1) Verzija [${VERZIJA_DEFAULT}]: " VERZIJA
VERZIJA="${VERZIJA:-$VERZIJA_DEFAULT}"
if [ "$VERZIJA" != "$VERZIJA_DEFAULT" ]; then
    echo "$VERZIJA" > "$VER_FAJL"
    echo "   → VERSION ažuriran na: $VERZIJA"
fi
echo ""

# 2. Okruženje
echo "2) Okruženje:"
echo "   1) Production  (podrazumevano)"
echo "   2) Development"
read -p "   Izbor [1/2]: " OKR_IZBOR
OKR_IZBOR="${OKR_IZBOR:-1}"
echo ""

# 3. Platforma
echo "3) Platforma:"
echo "   1) Linux   (podrazumevano)"
echo "   2) Windows"
echo "   3) Obe"
read -p "   Izbor [1/2/3]: " PLATFORMA_IZBOR
PLATFORMA_IZBOR="${PLATFORMA_IZBOR:-1}"
echo ""

# 4. UPX
read -p "4) Kompresovati UPX-om? [d/N]: " UPX_IZBOR
UPX_IZBOR="${UPX_IZBOR:-n}"
echo ""

# 5. Build
read -p "5) Pokrenuti build? [D/n]: " BUILD_IZBOR
BUILD_IZBOR="${BUILD_IZBOR:-d}"
echo ""

# 6. Docker push
read -p "6) Push Docker image (Gitea + GitHub)? [d/N]: " DOCKER_IZBOR
DOCKER_IZBOR="${DOCKER_IZBOR:-n}"
echo ""

# ── Izračunaj vrednosti ──────────────────────────
if [ "$OKR_IZBOR" = "2" ]; then
    OKRUZENJE="development"
    VERZIJA_BUILD="dev-${VERZIJA}"
    LDFLAGS="-X main.Verzija=dev-${VERZIJA}"
    TRIMPATH=""
else
    OKRUZENJE="production"
    VERZIJA_BUILD="${VERZIJA}"
    LDFLAGS="-X main.Verzija=${VERZIJA} -s -w"
    TRIMPATH="-trimpath"
fi

case "$PLATFORMA_IZBOR" in
    2) PLATFORMA_NAZIV="Windows" ;;
    3) PLATFORMA_NAZIV="Linux + Windows" ;;
    *) PLATFORMA_NAZIV="Linux" ;;
esac

if [[ "$UPX_IZBOR"    =~ ^[dDyY] ]]; then UPX_NAZIV="da";  else UPX_NAZIV="ne";  fi
if [[ "$BUILD_IZBOR"  =~ ^[dDyY] ]]; then BUILD_NAZIV="da"; else BUILD_NAZIV="ne"; fi
if [[ "$DOCKER_IZBOR" =~ ^[dDyY] ]]; then DOCKER_NAZIV="da"; else DOCKER_NAZIV="ne"; fi

# ── Sažetak ──────────────────────────────────────
echo "──────────────────────────────────────────"
echo "  Verzija    : ${VERZIJA_BUILD}"
echo "  Okruženje  : ${OKRUZENJE}"
echo "  Platforma  : ${PLATFORMA_NAZIV}"
echo "  UPX        : ${UPX_NAZIV}"
echo "  Build      : ${BUILD_NAZIV}"
echo "  Docker     : ${DOCKER_NAZIV}"
echo "──────────────────────────────────────────"
echo ""
read -p "Pokrenuti? [D/n]: " POTVRDA
POTVRDA="${POTVRDA:-d}"
if [[ ! "$POTVRDA" =~ ^[dDyY] ]]; then
    echo "Otkazano."
    exit 0
fi
echo ""

# ── UPX: instaliraj ako treba ────────────────────
if [[ "$UPX_IZBOR" =~ ^[dDyY] ]] && [[ "$BUILD_IZBOR" =~ ^[dDyY] ]]; then
    if ! command -v upx &>/dev/null; then
        echo "→ UPX nije instaliran. Instaliram..."
        if command -v apt-get &>/dev/null; then
            sudo apt-get install -y upx
        elif command -v dnf &>/dev/null; then
            sudo dnf install -y upx
        elif command -v pacman &>/dev/null; then
            sudo pacman -S --noconfirm upx
        elif command -v brew &>/dev/null; then
            brew install upx
        else
            echo "   UPOZORENJE: Ne mogu da instaliram UPX — nepoznat menadžer paketa. Kompresija preskočena."
            UPX_IZBOR="n"
        fi
        echo ""
    fi
fi

# ── Build funkcija ───────────────────────────────
build_za() {
    local GOOS_VAL="$1"
    local NAZIV="$2"
    echo "→ Build ${GOOS_VAL}/amd64: ${NAZIV}"
    CGO_ENABLED=0 GOOS="${GOOS_VAL}" GOARCH=amd64 go build \
        -ldflags "${LDFLAGS}" \
        ${TRIMPATH} \
        -o "${NAZIV}" \
        ./cmd/ntech
    ls -lh "${NAZIV}"

    if [[ "$UPX_IZBOR" =~ ^[dDyY] ]] && command -v upx &>/dev/null; then
        echo "   Kompresovanje sa UPX..."
        upx --best "${NAZIV}"
        ls -lh "${NAZIV}"
    fi
}

# ── 5. Build ─────────────────────────────────────
if [[ "$BUILD_IZBOR" =~ ^[dDyY] ]]; then
    echo "=== Build ==="
    case "$PLATFORMA_IZBOR" in
        2)
            build_za "windows" "ntech.exe"
            ;;
        3)
            build_za "linux"   "ntech"     &
            PID_LINUX=$!
            build_za "windows" "ntech.exe" &
            PID_WIN=$!
            wait $PID_LINUX $PID_WIN
            ;;
        *)
            build_za "linux" "ntech"
            ;;
    esac
    echo ""
fi

# ── 6. Docker push ───────────────────────────────
if [[ "$DOCKER_IZBOR" =~ ^[dDyY] ]]; then
    echo "=== Docker ==="

    # Ako Linux binary nije izgrađen u ovom pozivu (build=ne ili platforma=Windows), izgradi ga
    LINUX_VEC_IZGRADJEN=0
    if [[ "$BUILD_IZBOR" =~ ^[dDyY] ]] && [ "$PLATFORMA_IZBOR" != "2" ]; then
        LINUX_VEC_IZGRADJEN=1
    fi

    if [ "$LINUX_VEC_IZGRADJEN" = "0" ]; then
        echo "→ Gradim Linux binary za Docker (sa UPX ako je uključen)..."
        # instaliraj UPX ako treba, a još nije instaliran
        if [[ "$UPX_IZBOR" =~ ^[dDyY] ]] && ! command -v upx &>/dev/null; then
            echo "→ UPX nije instaliran. Instaliram..."
            if command -v apt-get &>/dev/null; then
                sudo apt-get install -y upx
            elif command -v dnf &>/dev/null; then
                sudo dnf install -y upx
            elif command -v pacman &>/dev/null; then
                sudo pacman -S --noconfirm upx
            elif command -v brew &>/dev/null; then
                brew install upx
            else
                echo "   UPOZORENJE: Ne mogu da instaliram UPX — kompresija preskočena."
                UPX_IZBOR="n"
            fi
        fi
        build_za "linux" "ntech"
        echo ""
    fi

    echo "→ Build Docker image..."
    docker build --build-arg="VERZIJA=${VERZIJA}" \
        -t "${GITEA_IMAGE}:${VERZIJA}" \
        -t "${GITEA_IMAGE}:latest" \
        -t "${GITHUB_IMAGE}:${VERZIJA}" \
        -t "${GITHUB_IMAGE}:latest" \
        .

    echo "→ Push na Gitea..."
    docker push "${GITEA_IMAGE}:${VERZIJA}"
    docker push "${GITEA_IMAGE}:latest"

    echo "→ Push na GitHub..."
    docker push "${GITHUB_IMAGE}:${VERZIJA}"
    docker push "${GITHUB_IMAGE}:latest"
    echo ""
fi

echo "==> Gotovo! NTech v${VERZIJA_BUILD}"
