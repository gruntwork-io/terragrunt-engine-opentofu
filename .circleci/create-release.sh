#!/usr/bin/env bash
# Create Github release from release candidate tag
set -euo pipefail
set -x
RC_VERSION=${TAG}
VERSION=${TAG%-rc*}
RELEASE="${RELEASE:-release}"
export GH_TOKEN=${GW_GITHUB_OAUTH_TOKEN}

COMMITS=$(git log $(git describe --tags --abbrev=0 @^)..@ --pretty=format:"- %s")

cat << EOF > rc_release_notes.txt
## Release $RC_VERSION

### Changes
$COMMITS

EOF
# create release candidate pre-release
gh release create "${RC_VERSION}" --prerelease -F rc_release_notes.txt -t "${RC_VERSION}" release/*

# create release
cat << EOF > release_notes.txt
## Release $VERSION

### Changes
$COMMITS

EOF


