#!/usr/bin/env bash
#
# Imports the Apple Developer ID certificate into a temporary keychain so codesign can use it.
# Runs on macOS, keep it compatible with bash 3.2.
#
# Environment variables:
#   MACOS_CERTIFICATE: Developer ID certificate in P12 format, base64 encoded
#   MACOS_CERTIFICATE_PASSWORD: password of the P12 certificate
#   RUNNER_TEMP: directory for the keychain (optional)

set -euo pipefail

# Apple Developer ID G2 intermediate certificate, see https://www.apple.com/certificateauthority/
readonly APPLE_DEVELOPER_ID_CA_URL="https://www.apple.com/certificateauthority/DeveloperIDG2CA.cer"

function main {
	: "${MACOS_CERTIFICATE:?ERROR: MACOS_CERTIFICATE is a required environment variable}"
	: "${MACOS_CERTIFICATE_PASSWORD:?ERROR: MACOS_CERTIFICATE_PASSWORD is a required environment variable}"

	local tmp_dir="${RUNNER_TEMP:-}"
	if [[ -z "$tmp_dir" ]]; then
		tmp_dir="$(mktemp -d)"
	fi

	local -r keychain="$tmp_dir/signing.keychain-db"
	local keychain_pw
	keychain_pw="$(openssl rand -hex 16)"

	security create-keychain -p "$keychain_pw" "$keychain"
	# Keep the keychain unlocked for 6 hours, notarization can take a while.
	security set-keychain-settings -lut 21600 "$keychain"
	security unlock-keychain -p "$keychain_pw" "$keychain"

	# codesign looks up identities only in keychains on the user search list.
	local keychains=("$keychain")
	local line
	while IFS= read -r line; do
		line="${line//\"/}"
		line="${line#"${line%%[![:space:]]*}"}"
		if [[ -n "$line" ]]; then
			keychains+=("$line")
		fi
	done < <(security list-keychains -d user)
	security list-keychains -d user -s "${keychains[@]}"
	security default-keychain -s "$keychain"

	base64 --decode <<<"$MACOS_CERTIFICATE" |
		security import /dev/stdin -f pkcs12 -k "$keychain" -P "$MACOS_CERTIFICATE_PASSWORD" -T /usr/bin/codesign

	curl -sSfL -o "$tmp_dir/DeveloperIDG2CA.cer" "$APPLE_DEVELOPER_ID_CA_URL"
	sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain "$tmp_dir/DeveloperIDG2CA.cer"

	security set-key-partition-list -S apple-tool:,apple:,codesign: -s -k "$keychain_pw" "$keychain"

	security find-identity -v -p codesigning "$keychain"

	return 0
}

main "$@"
