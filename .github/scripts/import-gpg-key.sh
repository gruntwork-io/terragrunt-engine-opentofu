#!/usr/bin/env bash
#
# Imports the engine GPG signing key.
#
# Environment variables:
#   GW_ENGINE_GPG_KEY: engine GPG private key, base64 encoded

set -euo pipefail

function main {
	: "${GW_ENGINE_GPG_KEY:?ERROR: GW_ENGINE_GPG_KEY is a required environment variable}"

	base64 --decode <<<"$GW_ENGINE_GPG_KEY" | gpg --batch --import

	return 0
}

main "$@"
