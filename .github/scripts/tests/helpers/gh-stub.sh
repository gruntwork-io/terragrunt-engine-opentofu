#!/usr/bin/env bash
#
# Fake gh for the release script tests. Every call is appended to GH_STUB_LOG.
#
# Environment variables:
#   GH_STUB_RELEASE: JSON printed by `gh release view`, empty means the release does not exist
#   GH_STUB_TAG_REFS: refs printed by `gh api .../git/matching-refs/...`, one per line
#   GH_STUB_TAG_SHA: commit printed by `gh api .../commits/...`

set -euo pipefail

echo "gh $*" >>"${GH_STUB_LOG:?}"

case "$1 $2" in
"release view")
	if [[ -z "${GH_STUB_RELEASE:-}" ]]; then
		echo "release not found" >&2
		exit 1
	fi
	echo "$GH_STUB_RELEASE"
	;;
"release upload") ;;
"api "*/git/matching-refs/*)
	if [[ -n "${GH_STUB_TAG_REFS:-}" ]]; then
		echo "$GH_STUB_TAG_REFS"
	fi
	;;
"api "*/commits/*)
	echo "${GH_STUB_TAG_SHA:?}"
	;;
*)
	echo "gh stub: unexpected call: $*" >&2
	exit 1
	;;
esac
