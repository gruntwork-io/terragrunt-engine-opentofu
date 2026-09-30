#!/usr/bin/env bash
#
# Checks that VERSION is a draft release targeting a commit SHA and writes
# version and ref to GITHUB_OUTPUT.
#
# The target must be a SHA, not a branch, and an existing version tag must point at
# the same commit, so the published tag matches the commit the binaries are built from.
#
# Environment variables:
#   VERSION: version of the draft release, e.g. v0.2.0
#   GITHUB_OUTPUT: GitHub Actions output file
#   GH_TOKEN, GH_REPO: used by gh

set -euo pipefail

function main {
	: "${VERSION:?ERROR: VERSION is a required environment variable}"
	: "${GITHUB_OUTPUT:?ERROR: GITHUB_OUTPUT is a required environment variable}"

	if [[ ! "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
		echo "::error::Version $VERSION is not a valid semver tag like v0.2.0"
		exit 1
	fi

	local release_json
	if ! release_json="$(gh release view "$VERSION" --json isDraft,targetCommitish)"; then
		echo "::error::No release found for $VERSION. Create a draft release first."
		exit 1
	fi

	if [[ "$(jq -r '.isDraft' <<<"$release_json")" != "true" ]]; then
		echo "::error::Release $VERSION is already published. Only draft releases can be updated."
		exit 1
	fi

	local ref
	ref="$(jq -r '.targetCommitish' <<<"$release_json")"
	if [[ ! "$ref" =~ ^[0-9a-f]{40}$ ]]; then
		echo "::error::Draft release target must be a full commit SHA, got '$ref'. Edit the draft and select a specific commit."
		exit 1
	fi

	"$(dirname "${BASH_SOURCE[0]}")/check-version-tag.sh" "$VERSION" "$ref"

	{
		printf 'version=%s\n' "$VERSION"
		printf 'ref=%s\n' "$ref"
	} >>"$GITHUB_OUTPUT"

	echo "Draft release $VERSION targets $ref"

	return 0
}

main "$@"
