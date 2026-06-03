#!/bin/bash
VERSION=${1:-"dev"}
echo "Buildovanje NTech v$VERSION..."
GOARCH=amd64 GOOS=linux go build -ldflags "-X main.Verzija=$VERSION -s -w" -o ntech ./cmd/ntech
if [ $? -eq 0 ]; then
    echo "Build završen: ntech v$VERSION"
    ls -lh ntech
else
    echo "Build neuspešan"
    exit 1
fi

