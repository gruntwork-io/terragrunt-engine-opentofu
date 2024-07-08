#!/usr/bin/env bash
# Create Github release from release candidate tag
set -euo pipefail

export GH_TOKEN=${GW_GITHUB_OAUTH_TOKEN}

readonly REPO_OWNER="${REPO_OWNER:-gruntwork-io}"
readonly REPO_NAME="${REPO_NAME:-terragrunt-engine-opentofu}"
readonly MAX_RETRIES=${MAX_RETRIES:-10}
readonly RETRY_INTERVAL=${RETRY_INTERVAL:-20}

readonly RC_VERSION=${TAG}
readonly VERSION=${TAG%-rc*}
readonly RELEASE="${RELEASE:-release}"
readonly COMMITS=$(git log $(git describe --tags --abbrev=0 @^)..@ --pretty=format:"- %s")

function create_rc_release_notes() {
  cat << EOF > rc_release_notes.txt
## Pre-Release $RC_VERSION

### Changes
$COMMITS

EOF
}

function create_release_notes() {
  cat << EOF > release_notes.txt
## Release $VERSION

### Changes
$COMMITS

EOF
}

function get_release_response() {
  local -r release_tag=$1

  # First try using the gh CLI
  local response
  if response=$(gh api -H "Accept: application/vnd.github+json" -H "X-GitHub-Api-Version: 2022-11-28" \
    "/repos/$REPO_OWNER/$REPO_NAME/releases/tags/$release_tag" 2> /dev/null); then
    echo "$response"
  else
    # Fallback to using curl if gh CLI fails
    response=$(curl -s \
      -H "Accept: application/vnd.github.v3+json" \
      -H "Authorization: token $GH_TOKEN" \
      -H "X-GitHub-Api-Version: 2022-11-28" \
      "https://api.github.com/repos/$REPO_OWNER/$REPO_NAME/releases/tags/$release_tag")
    echo "$response"
  fi
}

function check_release_exists() {
  local -r release_response=$1

  if jq -e '.message == "Not Found"' <<< "$release_response" > /dev/null; then
    echo "Release $CIRCLE_TAG not found. Exiting..."
    exit 1
  fi
}

function get_release_id() {
  local -r release_response=$1

  echo "$release_response" | jq -r '.id'
}

function get_asset_urls() {
  local -r release_response=$1

  echo "$release_response" | jq -r '.assets[].browser_download_url'
}

function verify_and_reupload_assets() {
  local -r release_version=$1
  local -r asset_dir=$2

  # Get the list of assets for the release
  local assets
  assets=$(gh release view "$release_version" --json assets --jq '.assets[].name')

  if [ -z "$assets" ]; then
    echo "No assets found for release $release_version"
    return
  fi

  for asset in $assets; do
    for ((i=0; i<MAX_RETRIES; i++)); do
      if ! gh release download "$release_version" --pattern "$asset" --dir /tmp > /dev/null 2>&1; then
        echo "Failed to download the asset $asset. Uploading..."

        # Re-upload the asset
        if ! gh release upload --clobber "$release_version" "${asset_dir}/${asset}"; then
          echo "Failed to upload $asset"
          sleep $RETRY_INTERVAL
        else
          echo "Successfully uploaded $asset"
          break
        fi
      else
        echo "Successfully checked the asset $asset"
        rm -f "/tmp/$asset"
        break
      fi
    done

    if (( i == MAX_RETRIES )); then
      echo "Failed to process the asset $asset after $MAX_RETRIES retries. Exiting..."
      exit 1
    fi
  done
}

function check_github_release() {
  local retries=0
  local release_tag=$1

  while [ $retries -lt $MAX_RETRIES ]; do
    if gh api -H "Accept: application/vnd.github+json" -H "X-GitHub-Api-Version: 2022-11-28" \
      "/repos/$REPO_OWNER/$REPO_NAME/releases" | jq -e --arg tag "$release_tag" '.[] | select(.tag_name == $tag)' > /dev/null; then
      echo "Release $release_tag found."
      return 0
    else
      echo "Release $release_tag not found. Retrying in $RETRY_INTERVAL seconds..."
      ((retries++))
      sleep $RETRY_INTERVAL
    fi
  done

  echo "Failed to find release $release_tag after $MAX_RETRIES retries. Exiting..."
  exit 1
}

function download_release_assets() {
  local -r release_tag=$1
  local -r download_dir=$2

  mkdir -p "$download_dir"

  assets=$(gh release view "$release_tag" --json assets --jq '.assets[].name')
  for asset in $assets; do
    gh release download "$release_tag" --pattern "$asset" -D "$download_dir"
  done
}

function main() {
  create_rc_release_notes
  # check if rc release exists, create if missing
  if ! gh release view "${RC_VERSION}" > /dev/null 2>&1; then
    gh release create "${RC_VERSION}" --prerelease -F rc_release_notes.txt -t "${RC_VERSION}" release/*
  fi
  check_github_release "${RC_VERSION}"
  verify_and_reupload_assets "${RC_VERSION}" "release"

  # download rc assets
  download_release_assets "$RC_VERSION" "release-bin"

  # create full release
  create_release_notes
  # check if release exists, create if missing
  if ! gh release view "${VERSION}" > /dev/null 2>&1; then
    gh release create "${VERSION}" -F release_notes.txt -t "${VERSION}" release-bin/*
  fi
  check_github_release "${VERSION}"
  verify_and_reupload_assets "${VERSION}" "release-bin"
}

main "$@"
