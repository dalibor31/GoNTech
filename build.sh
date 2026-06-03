Napravi build.sh skriptu u korenu projekta sa sledećim sadržajem:

#!/bin/bash
VERSION=${1:-"dev"}
echo "Buildovanje NTech v$VERSION..."
CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build -ldflags "-X main.Verzija=$VERSION -s -w" -o ntech ./cmd/ntech
if [ $? -eq 0 ]; then
    echo "Build završen: ntech v$VERSION"
    ls -lh ntech
else
    echo "Build neuspešan"
    exit 1
fi


