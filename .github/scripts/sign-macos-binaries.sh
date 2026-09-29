#!/usr/bin/env bash
#
# Signs and notarizes the macOS binaries in bin/ with gon, using .gon_<arch>.hcl.
# Runs on macOS, keep it compatible with bash 3.2.
#
# Environment variables:
#   AC_PASSWORD: Apple notarization password
#   AC_PROVIDER: Apple notarization provider

set -euo pipefail

readonly NAME="terragrunt-iac-engine-opentofu"

function main {
	: "${AC_PASSWORD:?ERROR: AC_PASSWORD is a required environment variable}"
	: "${AC_PROVIDER:?ERROR: AC_PROVIDER is a required environment variable}"

	local arch
	for arch in amd64 arm64; do
		gon -log-level=info ".gon_${arch}.hcl"

		# gon zips the signed binary, use it in place of the unsigned one.
		unzip -o "${NAME}_${arch}.zip"
		mv "${NAME}_darwin_${arch}" bin/

		codesign -dv --verbose=4 "bin/${NAME}_darwin_${arch}"
	done

	return 0
}

main "$@"
