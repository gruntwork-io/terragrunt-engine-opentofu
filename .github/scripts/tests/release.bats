#!/usr/bin/env bats

readonly BUILT_SHA="1111111111111111111111111111111111111111"
readonly OTHER_SHA="2222222222222222222222222222222222222222"

setup() {
	SCRIPTS="${BATS_TEST_DIRNAME}/.."

	mkdir -p "${BATS_TEST_TMPDIR}/bin"
	cp "${BATS_TEST_DIRNAME}/helpers/gh-stub.sh" "${BATS_TEST_TMPDIR}/bin/gh"
	chmod +x "${BATS_TEST_TMPDIR}/bin/gh"
	export PATH="${BATS_TEST_TMPDIR}/bin:${PATH}"

	export GH_STUB_LOG="${BATS_TEST_TMPDIR}/gh.log"
	export GITHUB_OUTPUT="${BATS_TEST_TMPDIR}/output"
	touch "$GH_STUB_LOG" "$GITHUB_OUTPUT"

	export GH_TOKEN=fake
	export GH_REPO=gruntwork-io/terragrunt-engine-opentofu
	export VERSION=v0.2.0
	export EXPECTED_REF="$BUILT_SHA"

	RELEASE_DIR="${BATS_TEST_TMPDIR}/release"
	mkdir -p "$RELEASE_DIR"
	touch "$RELEASE_DIR/engine.zip" "$RELEASE_DIR/engine_SHA256SUMS" "$RELEASE_DIR/engine_SHA256SUMS.sig"
}

draft_targeting() {
	export GH_STUB_RELEASE="{\"isDraft\":true,\"targetCommitish\":\"$1\",\"url\":\"https://example.invalid/draft\"}"
}

tag_at() {
	export GH_STUB_TAG_REFS="refs/tags/${VERSION}"
	export GH_STUB_TAG_SHA="$1"
}

# check-version-tag.sh

@test "tag check passes when the tag does not exist" {
	run "$SCRIPTS/check-version-tag.sh" "$VERSION" "$BUILT_SHA"
	[ "$status" -eq 0 ]
	[[ "$output" == *"does not exist yet"* ]]
}

@test "tag check ignores tags that only share the version prefix" {
	export GH_STUB_TAG_REFS="refs/tags/${VERSION}-rc1"
	run "$SCRIPTS/check-version-tag.sh" "$VERSION" "$BUILT_SHA"
	[ "$status" -eq 0 ]
	[[ "$output" == *"does not exist yet"* ]]
}

@test "tag check passes when the tag points at the expected commit" {
	tag_at "$BUILT_SHA"
	run "$SCRIPTS/check-version-tag.sh" "$VERSION" "$BUILT_SHA"
	[ "$status" -eq 0 ]
}

@test "tag check fails when the tag points at another commit" {
	tag_at "$OTHER_SHA"
	run "$SCRIPTS/check-version-tag.sh" "$VERSION" "$BUILT_SHA"
	[ "$status" -eq 1 ]
	[[ "$output" == *"already exists at $OTHER_SHA"* ]]
}

# validate-draft-release.sh

@test "validate rejects a version that is not semver" {
	export VERSION=0.2.0
	run "$SCRIPTS/validate-draft-release.sh"
	[ "$status" -eq 1 ]
	[[ "$output" == *"not a valid semver tag"* ]]
}

@test "validate rejects a published release" {
	export GH_STUB_RELEASE="{\"isDraft\":false,\"targetCommitish\":\"$BUILT_SHA\"}"
	run "$SCRIPTS/validate-draft-release.sh"
	[ "$status" -eq 1 ]
	[[ "$output" == *"already published"* ]]
}

@test "validate rejects a draft that targets a branch" {
	draft_targeting main
	run "$SCRIPTS/validate-draft-release.sh"
	[ "$status" -eq 1 ]
	[[ "$output" == *"must be a full commit SHA"* ]]
}

@test "validate rejects a draft whose existing tag points at another commit" {
	draft_targeting "$BUILT_SHA"
	tag_at "$OTHER_SHA"
	run "$SCRIPTS/validate-draft-release.sh"
	[ "$status" -eq 1 ]
	[[ "$output" == *"already exists at $OTHER_SHA"* ]]
	[ ! -s "$GITHUB_OUTPUT" ]
}

@test "validate writes version and ref for a valid draft" {
	draft_targeting "$BUILT_SHA"
	run "$SCRIPTS/validate-draft-release.sh"
	[ "$status" -eq 0 ]
	grep -qx "version=$VERSION" "$GITHUB_OUTPUT"
	grep -qx "ref=$BUILT_SHA" "$GITHUB_OUTPUT"
}

# upload-release-assets.sh

@test "upload refuses a release published during the run" {
	export GH_STUB_RELEASE="{\"isDraft\":false,\"targetCommitish\":\"$BUILT_SHA\",\"url\":\"u\"}"
	run "$SCRIPTS/upload-release-assets.sh" "$RELEASE_DIR"
	[ "$status" -eq 1 ]
	! grep -q "release upload" "$GH_STUB_LOG"
}

@test "upload refuses a draft whose target changed during the run" {
	draft_targeting "$OTHER_SHA"
	run "$SCRIPTS/upload-release-assets.sh" "$RELEASE_DIR"
	[ "$status" -eq 1 ]
	[[ "$output" == *"binaries were built from $BUILT_SHA"* ]]
	! grep -q "release upload" "$GH_STUB_LOG"
}

@test "upload refuses a tag created at another commit during the run" {
	draft_targeting "$BUILT_SHA"
	tag_at "$OTHER_SHA"
	run "$SCRIPTS/upload-release-assets.sh" "$RELEASE_DIR"
	[ "$status" -eq 1 ]
	! grep -q "release upload" "$GH_STUB_LOG"
}

@test "upload attaches zips, checksums and signature to a valid draft" {
	draft_targeting "$BUILT_SHA"
	run "$SCRIPTS/upload-release-assets.sh" "$RELEASE_DIR"
	[ "$status" -eq 0 ]
	grep -q "release upload $VERSION $RELEASE_DIR/engine.zip $RELEASE_DIR/engine_SHA256SUMS $RELEASE_DIR/engine_SHA256SUMS.sig --clobber" "$GH_STUB_LOG"
}
