#!/usr/bin/env bash
#
# Fails when go mod tidy changes go.mod or go.sum.

set -euo pipefail

function main {
	go mod tidy

	if [[ -n "$(git status --porcelain)" ]]; then
		echo "::error::go mod tidy made changes to go.mod or go.sum"
		git diff
		exit 1
	fi

	return 0
}

main "$@"
