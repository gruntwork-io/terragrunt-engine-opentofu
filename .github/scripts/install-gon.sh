#!/usr/bin/env bash
#
# Installs gon, used to sign and notarize macOS binaries, into /usr/local/bin.
#
# Usage: install-gon.sh <version>

set -euo pipefail

function main {
	local -r version="${1:?ERROR: gon version is required}"

	local tmp_dir
	tmp_dir="$(mktemp -d)"

	curl -sSfL -o "$tmp_dir/gon.zip" "https://github.com/Bearer/gon/releases/download/${version}/gon_macos.zip"
	unzip -o "$tmp_dir/gon.zip" gon -d "$tmp_dir"
	sudo install -m 0755 "$tmp_dir/gon" /usr/local/bin/gon
	rm -rf "$tmp_dir"

	gon --version

	return 0
}

main "$@"
