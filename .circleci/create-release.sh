#!/usr/bin/env bash
# Create Github release from release candidate tag
set -euo pipefail

VERSION=${TAG%-rc*}
RELEASE="${RELEASE:-release}"

COMMITS=$(git log $(git describe --tags --abbrev=0 @^)..@ --pretty=format:"- %s")

cat << EOF > release_notes.txt
## Release $VERSION

### Changes
$COMMITS

EOF

cd "${RELEASE}"
gh release create "${VERSION}" \
  -F release_notes.txt \
  -t "Release ${VERSION}" *
