#!/usr/bin/env bash
# This script is used to sign the release files with the GPG key
set -euo pipefail

BIN="${BIN:-bin}"
RELEASE="${RELEASE:-release}"
NAME="${NAME:-opentofu}"
TYPE="rpc"

KEY_ID=$(gpg --list-keys --with-colons | awk -F: '/^pub/{print $5}')

mkdir -p "${RELEASE}"
cd "${BIN}"
for file in *; do
  # Extract the OS and ARCH from the file name
  if [[ $file =~ terragrunt-iac-engine-${NAME}_([^_]+)_([^_]+) ]]; then
  OS="${BASH_REMATCH[1]}"
  ARCH="${BASH_REMATCH[2]}"

  # Set the binary and archive names
  BINARY_NAME="terragrunt-iac-engine-${NAME}_${TAG}"
  mv "${file}" "${BINARY_NAME}"
  ARCHIVE_NAME="terragrunt-iac-engine-${NAME}_${TYPE}_${TAG}_${OS}_${ARCH}.zip"

  # Create the zip archive
  zip "../${RELEASE}/${ARCHIVE_NAME}" "${BINARY_NAME}"
  fi
done
cd "../${RELEASE}"
shasum -a 256 *.zip > "terragrunt-iac-engine-${NAME}_${TYPE}_${TAG}_SHA256SUMS"
echo "${GW_ENGINE_GPG_KEY_PW}" | gpg --batch --yes --pinentry-mode loopback --passphrase-fd 0 --default-key "${KEY_ID}" --detach-sign "terragrunt-iac-engine-${NAME}_${TYPE}_${TAG}_SHA256SUMS"
