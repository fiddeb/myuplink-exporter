#!/bin/sh
# Build a self-contained local app context from this checkout's current sources.
set -eu

if [ "$#" -ne 1 ]; then
    echo "Usage: sh scripts/package-homeassistant.sh DESTINATION" >&2
    exit 1
fi

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
app="$root/homeassistant/myuplink_cloud"
mkdir -p -- "$1"
destination=$(CDPATH= cd -- "$1" && pwd)
if [ "$destination" = "$root" ] || [ "$destination" = "$app" ]; then
    echo "Choose a separate destination for the packaged app" >&2
    exit 1
fi

archive=$(mktemp "$destination/.exporter-source.XXXXXX")
trap 'rm -f -- "$archive"' 0
trap 'exit 1' HUP INT TERM
# Copy only build inputs, never logs, credentials, Git history or local caches.
(
    cd -- "$root"
    COPYFILE_DISABLE=1 tar -cf "$archive" LICENSE go.mod go.sum ./*.go config/*.go myuplink/*.go
)
for file in Dockerfile config.yaml run.py config.alloy DOCS.md .dockerignore .gitignore; do
    cp -- "$app/$file" "$destination/$file"
done
mv -- "$archive" "$destination/exporter-source.tar"
echo "Packaged current exporter sources in $destination"
