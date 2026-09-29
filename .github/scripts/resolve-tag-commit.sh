#!/usr/bin/env bash
#
# Prints the commit the version tag points at. Fails when the tag does not exist.
#
# Usage: resolve-tag-commit.sh <version>
#
# Environment variables:
#   GH_REPO: repository, e.g. gruntwork-io/terragrunt-engine-opentofu
#   GH_TOKEN: used by gh

set -euo pipefail

function main {
	local -r version="${1:?ERROR: version is required}"

	: "${GH_REPO:?ERROR: GH_REPO is a required environment variable}"

	# matching-refs is a prefix match, so look for the exact tag ref.
	local refs
	refs="$(gh api "repos/${GH_REPO}/git/matching-refs/tags/${version}" --jq '.[].ref')"

	if ! grep -Fxq "refs/tags/${version}" <<<"$refs"; then
		echo "ERROR: tag $version does not exist" >&2
		exit 1
	fi

	# The commits API resolves annotated tags to the commit they point at.
	gh api "repos/${GH_REPO}/commits/refs/tags/${version}" --jq '.sha'

	return 0
}

main "$@"
