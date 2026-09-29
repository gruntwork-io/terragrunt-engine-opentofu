#!/usr/bin/env bash
#
# Checks that VERSION is a published pre-release and writes version and the commit its tag
# points at (ref) to GITHUB_OUTPUT. The release files are built from that commit.
#
# Environment variables:
#   VERSION: version of the pre-release, e.g. v0.2.0
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
	if ! release_json="$(gh release view "$VERSION" --json isDraft,isPrerelease)"; then
		echo "::error::No release found for $VERSION. Publish it as a pre-release first."
		exit 1
	fi

	if [[ "$(jq -r '.isDraft' <<<"$release_json")" == "true" ]]; then
		echo "::error::Release $VERSION is a draft. Publish it as a pre-release to build it."
		exit 1
	fi

	if [[ "$(jq -r '.isPrerelease' <<<"$release_json")" != "true" ]]; then
		echo "::error::Release $VERSION is a full release. Files are attached only to pre-releases."
		exit 1
	fi

	local ref
	if ! ref="$("$(dirname "${BASH_SOURCE[0]}")/resolve-tag-commit.sh" "$VERSION")"; then
		echo "::error::Tag $VERSION does not exist."
		exit 1
	fi

	{
		printf 'version=%s\n' "$VERSION"
		printf 'ref=%s\n' "$ref"
	} >>"$GITHUB_OUTPUT"

	echo "Pre-release $VERSION: tag points at $ref"

	return 0
}

main "$@"
