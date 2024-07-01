#!/usr/bin/env bash
# Create Github release from release candidate tag
set -euo pipefail
set -x

readonly RC_VERSION=${TAG}
readonly VERSION=${TAG%-rc*}
readonly RELEASE="${RELEASE:-release}"
export GH_TOKEN=${GW_GITHUB_OAUTH_TOKEN}
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

  curl -s \
    -H "Accept: application/vnd.github.v3+json" \
    -H "Authorization: token $GITHUB_OAUTH_TOKEN" \
    -H "X-GitHub-Api-Version: 2022-11-28" \
    "https://api.github.com/repos/$REPO_OWNER/$REPO_NAME/releases/tags/$release_tag"
}

function check_release_exists() {
  local -r release_response=$1

  if jq -e '.message == "Not Found"' <<< "$release_response" > /dev/null; then
    echo "Release $CIRCLE_TAG not found. Exiting..."
    exit 1
  fi
}

get_release_id() {
  local -r release_response=$1

  echo "$release_response" | jq -r '.id'
}

get_asset_urls() {
  local -r release_response=$1

  echo "$release_response" | jq -r '.assets[].browser_download_url'
}

verify_and_reupload_assets() {
  local -r release_version=$1
  local -r asset_dir=$2

  local release_response
  release_response=$(get_release_response "$release_version")

  check_release_exists "$release_response"
  local release_id
  release_id=$(get_release_id "$release_response")
  local asset_urls
  asset_urls=$(get_asset_urls "$release_response")

  for asset_url in $asset_urls; do
    local asset_name
    asset_name=$(basename "$asset_url")

    for ((i=0; i<MAX_RETRIES; i++)); do
      if ! curl -sILf "$asset_url" > /dev/null; then
        echo "Failed to download the asset $asset_name. Retrying..."

        # Delete the asset
        local asset_id
        asset_id=$(jq -r --arg asset_name "$asset_name" '.assets[] | select(.name == $asset_name) | .id' <<< "$release_response")
        curl -s -L -XDELETE \
          -H "Accept: application/vnd.github.v3+json" \
          -H "Authorization: token $GITHUB_OAUTH_TOKEN" \
          -H "X-GitHub-Api-Version: 2022-11-28" \
          "https://api.github.com/repos/$REPO_OWNER/$REPO_NAME/releases/assets/$asset_id" > /dev/null

        # Re-upload the asset
        curl -s -L -XPOST \
          -H "Accept: application/vnd.github.v3+json" \
          -H "Authorization: token $GITHUB_OAUTH_TOKEN" \
          -H "X-GitHub-Api-Version: 2022-11-28" \
          -H "Content-Type: application/octet-stream" \
          --data-binary "@${asset_dir}/${asset_name}" \
          "https://uploads.github.com/repos/$REPO_OWNER/$REPO_NAME/releases/$release_id/assets?name=${asset_name}" > /dev/null
      else
        echo "Successfully checked the asset $asset_name"
        break
      fi
    done

    if (( i == MAX_RETRIES )); then
      echo "Failed to download the asset $asset_name after $MAX_RETRIES retries. Exiting..."
      exit 1
    fi
  done
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
    gh release create "${RC_VERSION}" --prerelease -F rc_release_notes.txt -t "${RC_VERSION}"
  fi
  verify_and_reupload_asset "${RC_VERSION}" "release"

  # download rc assets
  download_release_assets "$RC_VERSION" "release-bin"

  # create full release
  create_release_notes
  # check if release exists, create if missing
  if ! gh release view "${VERSION}" > /dev/null 2>&1; then
    gh release create "${VERSION}" -F release_notes.txt -t "${VERSION}" release-bin/*
  fi

  verify_and_reupload_asset "${VERSION}" "release-bin"
}

main "$@"
