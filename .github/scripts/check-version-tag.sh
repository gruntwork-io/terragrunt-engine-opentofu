#!/usr/bin/env bash
#
# Fails when the version tag already exists at a commit other than the expected one.
#
# GitHub ignores a draft release's target when its tag already exists, so publishing would
# release the existing tag instead of the commit the binaries were built from.
#
# Usage: check-version-tag.sh <version> <commit-sha>
#
# Environment variables:
#   GH_REPO: repository, e.g. gruntwork-io/terragrunt-engine-opentofu
#   GH_TOKEN: used by gh

set -euo pipefail

function main {
	local -r version="${1:?ERROR: version is required}"
	local -r expected_sha="${2:?ERROR: commit SHA is required}"

	: "${GH_REPO:?ERROR: GH_REPO is a required environment variable}"

	# matching-refs is a prefix match, so look for the exact tag ref.
	local refs
	refs="$(gh api "repos/${GH_REPO}/git/matching-refs/tags/${version}" --jq '.[].ref')"

	if ! grep -Fxq "refs/tags/${version}" <<<"$refs"; then
		echo "Tag $version does not exist yet, publishing creates it at $expected_sha"
		return 0
	fi

	# The commits API resolves annotated tags to the commit they point at.
	local tag_sha
	tag_sha="$(gh api "repos/${GH_REPO}/commits/refs/tags/${version}" --jq '.sha')"

	if [[ "$tag_sha" != "$expected_sha" ]]; then
		echo "::error::Tag $version already exists at $tag_sha, but the draft targets $expected_sha. GitHub publishes the existing tag, so delete the tag or set the draft target to $tag_sha."
		exit 1
	fi

	echo "Tag $version already exists at $expected_sha"

	return 0
}

main "$@"
