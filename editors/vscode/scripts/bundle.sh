#!/bin/sh
set -e
target=${1:-$(node -p 'process.platform + "-" + process.arch')}
case $target in
  darwin-arm64) goos=darwin goarch=arm64 ;;
  darwin-x64) goos=darwin goarch=amd64 ;;
  linux-arm64) goos=linux goarch=arm64 ;;
  linux-x64) goos=linux goarch=amd64 ;;
  win32-arm64) goos=windows goarch=arm64 exe=.exe ;;
  win32-x64) goos=windows goarch=amd64 exe=.exe ;;
  *) echo "unsupported target: $target" >&2; exit 1 ;;
esac
here=$(cd "$(dirname "$0")/.." && pwd)
rm -rf "$here/bin"
mkdir -p "$here/bin"
cd "$here/../.."
version=$(git describe --tags --always --dirty)
CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch go build -trimpath \
  -ldflags "-s -w -X github.com/dwwescalelol/awf-cli/internal/build.Version=$version" \
  -o "$here/bin/awf$exe" ./cmd/awf
