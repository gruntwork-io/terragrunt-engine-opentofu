#!/usr/bin/env bash
#
# Attaches the packaged release files to the VERSION draft release.
#
# Fails without uploading when the release was published, or when the draft target changed
# after the binaries were built, so the files always match the commit the tag will point at.
#
# Usage: upload-release-assets.sh <release-dir>
#
# Environment variables:
#   VERSION: version of the draft release, e.g. v0.2.0
#   EXPECTED_REF: commit SHA the binaries were built from
#   GITHUB_STEP_SUMMARY: GitHub Actions summary file (optional)
#   GH_TOKEN, GH_REPO: used by gh

set -euo pipefail

function main {
	local -r release_dir="${1:?ERROR: release dir is required}"

	: "${VERSION:?ERROR: VERSION is a required environment variable}"
	: "${EXPECTED_REF:?ERROR: EXPECTED_REF is a required environment variable}"

	local release_json
	release_json="$(gh release view "$VERSION" --json isDraft,targetCommitish,url)"

	if [[ "$(jq -r '.isDraft' <<<"$release_json")" != "true" ]]; then
		echo "::error::Release $VERSION was published while the workflow was running. Files not attached."
		exit 1
	fi

	local target
	target="$(jq -r '.targetCommitish' <<<"$release_json")"
	if [[ "$target" != "$EXPECTED_REF" ]]; then
		echo "::error::Draft $VERSION now targets $target, but the binaries were built from $EXPECTED_REF. Files not attached, run the workflow again."
		exit 1
	fi

	# Raw binaries are listed in SHA256SUMS, but only the zips are attached.
	gh release upload "$VERSION" \
		"$release_dir"/*.zip \
		"$release_dir"/*_SHA256SUMS \
		"$release_dir"/*_SHA256SUMS.sig \
		--clobber

	if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
		{
			echo "## Files attached to draft release $VERSION"
			echo
			gh release view "$VERSION" --json assets -q '.assets[] | "- \(.name)"'
			echo
			echo "Review the draft and publish it: $(jq -r '.url' <<<"$release_json")"
		} >>"$GITHUB_STEP_SUMMARY"
	fi

	return 0
}

main "$@"
