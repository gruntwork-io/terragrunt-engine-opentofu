#!/usr/bin/env bash
#
# Writes the Go build and module cache directories to GITHUB_OUTPUT as go-build and go-mod.
#
# Environment variables:
#   GITHUB_OUTPUT: GitHub Actions output file

set -euo pipefail

function main {
	: "${GITHUB_OUTPUT:?ERROR: GITHUB_OUTPUT is a required environment variable}"

	{
		printf 'go-build=%s\n' "$(go env GOCACHE)"
		printf 'go-mod=%s\n' "$(go env GOMODCACHE)"
	} >>"$GITHUB_OUTPUT"

	return 0
}

main "$@"
