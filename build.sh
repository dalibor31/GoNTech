#!/bin/bash
set -e

# ──────────────────────────────────────────────
#  Verzija
# ──────────────────────────────────────────────
read -p "Verzija (npr. 0.1.1): " VERZIJA
VERZIJA=${VERZIJA:-"dev"}

# ──────────────────────────────────────────────
#  Okruženje
# ──────────────────────────────────────────────
echo ""
echo "Okruženje:"
echo "  1) production"
echo "  2) development"
read -p "Izbor [1/2, podrazumevano 1]: " OKR_IZBOR
OKR_IZBOR=${OKR_IZBOR:-1}

if [ "$OKR_IZBOR" = "2" ]; then
    OKRUZENJE="development"
    LDFLAGS="-X main.Verzija=dev-${VERZIJA}"
    NAZIV="ntech-dev-${VERZIJA}"
else
    OKRUZENJE="production"
    LDFLAGS="-X main.Verzija=${VERZIJA} -s -w"
    NAZIV="ntech-${VERZIJA}"
fi

# ──────────────────────────────────────────────
#  Ciljni OS
# ──────────────────────────────────────────────
echo ""
echo "Ciljni OS:"
echo "  1) Linux   (amd64)"
echo "  2) Windows (amd64)"
read -p "Izbor [1/2, podrazumevano 1]: " OS_IZBOR
OS_IZBOR=${OS_IZBOR:-1}

if [ "$OS_IZBOR" = "2" ]; then
    GOOS_VAL="windows"
    NAZIV="${NAZIV}.exe"
else
    GOOS_VAL="linux"
fi

# ──────────────────────────────────────────────
#  UPX kompresija
# ──────────────────────────────────────────────
echo ""
UPX_DOSTUPAN=false
if command -v upx &>/dev/null; then
    UPX_DOSTUPAN=true
    read -p "Kompresovati UPX-om? [d/N]: " UPX_IZBOR
else
    echo "UPX nije instaliran — kompresija preskočena."
    UPX_IZBOR="n"
fi

# ──────────────────────────────────────────────
#  Sažetak pre builda
# ──────────────────────────────────────────────
echo ""
echo "──────────────────────────────────────────"
echo "  Okruženje : ${OKRUZENJE}"
echo "  Verzija   : ${VERZIJA}"
echo "  OS        : ${GOOS_VAL}/amd64"
echo "  Izlaz     : ${NAZIV}"
if [ "$UPX_DOSTUPAN" = true ] && [[ "$UPX_IZBOR" =~ ^[dDyY] ]]; then
    echo "  UPX       : da"
else
    echo "  UPX       : ne"
fi
echo "──────────────────────────────────────────"
echo ""
read -p "Pokrenuti build? [D/n]: " POTVRDA
POTVRDA=${POTVRDA:-"d"}
if [[ ! "$POTVRDA" =~ ^[dDyY] ]]; then
    echo "Build otkazan."
    exit 0
fi

# ──────────────────────────────────────────────
#  Build
# ──────────────────────────────────────────────
echo ""
echo "Buildovanje..."
CGO_ENABLED=0 GOARCH=amd64 GOOS=${GOOS_VAL} go build \
    -ldflags "${LDFLAGS}" \
    -o "${NAZIV}" \
    ./cmd/ntech

echo "Build završen: ${NAZIV}"
ls -lh "${NAZIV}"

# ──────────────────────────────────────────────
#  UPX
# ──────────────────────────────────────────────
if [ "$UPX_DOSTUPAN" = true ] && [[ "$UPX_IZBOR" =~ ^[dDyY] ]]; then
    echo ""
    echo "Kompresovanje sa UPX..."
    upx --best "${NAZIV}"
    echo "Nakon kompresije:"
    ls -lh "${NAZIV}"
fi

echo ""
echo "Gotovo."
