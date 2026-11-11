#!/bin/sh
# Build mcp-pipe into resources/<goos>-<goarch>/bin for the desktop app.
#
#   ./scripts/bundle-mcp-pipe.sh                 # this machine
#   ./scripts/bundle-mcp-pipe.sh darwin/universal
#   ./scripts/bundle-mcp-pipe.sh linux/amd64,linux/arm64
set -eu

cd "$(dirname "$0")/.."

version="$(sed -n 's/.*"version": "\([^"]*\)".*/\1/p' mygo.json)"
targets="${1:-$(go env GOOS)/$(go env GOARCH)}"

build_one() {
	goos="$1"
	goarch="$2"
	ext=""
	if [ "$goos" = "windows" ]; then
		ext=".exe"
	fi
	dir="resources/${goos}-${goarch}/bin"
	mkdir -p "$dir"
	CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
		go build -trimpath -ldflags "-s -w -X github.com/tk103331/mymcp/internal/gateway.Version=${version}" \
		-o "${dir}/mcp-pipe${ext}" ./cmd/mcp-pipe
	if [ "$goos" != "windows" ]; then
		chmod 0755 "${dir}/mcp-pipe${ext}"
	fi
	echo "bundled ${dir}/mcp-pipe${ext}"
}

for target in $(printf '%s' "$targets" | tr ',' ' '); do
	case "$target" in
	darwin/universal)
		build_one darwin arm64
		build_one darwin amd64
		;;
	*/*)
		goos="${target%%/*}"
		goarch="${target#*/}"
		build_one "$goos" "$goarch"
		;;
	*)
		echo "invalid target: $target (want GOOS/GOARCH)" >&2
		exit 2
		;;
	esac
done
