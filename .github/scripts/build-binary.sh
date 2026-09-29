#!/usr/bin/env bash
#
# Builds the engine binary for one platform into bin/.
#
# Usage: build-binary.sh <os> <arch> <version>

set -euo pipefail

readonly NAME="terragrunt-iac-engine-opentofu"

function main {
	local -r os="${1:?ERROR: os is required}"
	local -r arch="${2:?ERROR: arch is required}"
	local -r version="${3:?ERROR: version is required}"

	local ext=""
	if [[ "$os" == "windows" ]]; then
		ext=".exe"
	fi

	GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 go build \
		-o "bin/${NAME}_${os}_${arch}${ext}" \
		-ldflags "-X github.com/gruntwork-io/go-commons/version.Version=${version} -extldflags '-static'" \
		.

	return 0
}

main "$@"
