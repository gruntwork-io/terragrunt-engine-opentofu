#!/usr/bin/env bash
#
# Packages engine binaries into the release layout that Terragrunt downloads and verifies:
#
#   terragrunt-iac-engine-opentofu_rpc_<version>_<os>_<arch>[.exe]
#   terragrunt-iac-engine-opentofu_rpc_<version>_<os>_<arch>.zip
#   terragrunt-iac-engine-opentofu_rpc_<version>_SHA256SUMS
#   terragrunt-iac-engine-opentofu_rpc_<version>_SHA256SUMS.sig (detached GPG signature)
#
# Usage: package-release.sh <bin-dir> <out-dir> <version>
#
# <bin-dir> holds terragrunt-iac-engine-opentofu_<os>_<arch>[.exe] binaries.
# SHA256SUMS is signed with the default GPG key; GPG_PASSPHRASE is used when set.

set -euo pipefail

readonly NAME="terragrunt-iac-engine-opentofu"

function main {
	local -r bin_dir="${1:?ERROR: bin dir is required}"
	local -r out_dir="${2:?ERROR: out dir is required}"
	local -r version="${3:?ERROR: version is required}"
	local -r prefix="${NAME}_rpc_${version}"

	mkdir -p "$out_dir"
	rm -f "$out_dir/${prefix}"_*

	local src platform ext file
	for src in "$bin_dir/${NAME}"_*; do
		platform="$(basename "$src")"
		platform="${platform#"${NAME}_"}"
		ext=""
		if [[ "$platform" == *.exe ]]; then
			ext=".exe"
			platform="${platform%.exe}"
		fi

		file="${prefix}_${platform}${ext}"
		cp "$src" "$out_dir/$file"
		chmod +x "$out_dir/$file"
		(cd "$out_dir" && zip -q "${prefix}_${platform}.zip" "$file")
	done

	local -a passphrase_args=()
	if [[ -n "${GPG_PASSPHRASE:-}" ]]; then
		passphrase_args=(--passphrase-fd 0)
	fi

	cd "$out_dir"
	sha256sum "${prefix}"_* >"${prefix}_SHA256SUMS"
	gpg --batch --yes --pinentry-mode loopback "${passphrase_args[@]}" \
		--output "${prefix}_SHA256SUMS.sig" \
		--detach-sign "${prefix}_SHA256SUMS" <<<"${GPG_PASSPHRASE:-}"
	gpg --verify "${prefix}_SHA256SUMS.sig" "${prefix}_SHA256SUMS"

	ls -l
}

main "$@"
