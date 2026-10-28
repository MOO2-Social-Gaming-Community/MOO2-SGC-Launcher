#!/bin/sh
set -eu
cd "$(dirname "$0")"
mkdir -p out
for target in windows-amd64 linux-amd64 darwin-amd64 darwin-arm64; do
 OS=${target%-*}; ARCH=${target#*-}; NAME="moo2-manager-$target"
 [ "$OS" != windows ] || NAME="$NAME.exe"
 CGO_ENABLED=0 GOOS="$OS" GOARCH="$ARCH" go build -buildvcs=false -trimpath -ldflags='-s -w' -o "out/$NAME" .
done
