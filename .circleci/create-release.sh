#!/usr/bin/env bash
# Create Github release from release candidate tag
set -euo pipefail
set -x
readonly  RC_VERSION=${TAG}
readonly VERSION=${TAG%-rc*}
readonly RELEASE="${RELEASE:-release}"
readonly  GH_TOKEN=${GW_GITHUB_OAUTH_TOKEN}

readonly COMMITS=$(git log $(git describe --tags --abbrev=0 @^)..@ --pretty=format:"- %s")


cat << EOF > rc_release_notes.txt
## Release $RC_VERSION

### Changes
$COMMITS

EOF
# create release candidate pre-release
gh release create "${RC_VERSION}" --prerelease -F rc_release_notes.txt -t "${RC_VERSION}" release/*

# download pre-release files to folder release-bin to be used in release

# create release
cat << EOF > release_notes.txt
## Release $VERSION

### Changes
$COMMITS

EOF

function main {
}

main "$@"

}