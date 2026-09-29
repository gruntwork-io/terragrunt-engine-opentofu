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

release_is() {
	export GH_STUB_RELEASE="{\"isDraft\":$1,\"isPrerelease\":$2,\"url\":\"https://example.invalid/release\"}"
}

tag_at() {
	export GH_STUB_TAG_REFS="refs/tags/${VERSION}"
	export GH_STUB_TAG_SHA="$1"
}

# resolve-tag-commit.sh

@test "resolve fails when the tag does not exist" {
	run "$SCRIPTS/resolve-tag-commit.sh" "$VERSION"
	[ "$status" -eq 1 ]
}

@test "resolve ignores tags that only share the version prefix" {
	export GH_STUB_TAG_REFS="refs/tags/${VERSION}-rc1"
	run "$SCRIPTS/resolve-tag-commit.sh" "$VERSION"
	[ "$status" -eq 1 ]
}

@test "resolve prints the commit the tag points at" {
	tag_at "$BUILT_SHA"
	run "$SCRIPTS/resolve-tag-commit.sh" "$VERSION"
	[ "$status" -eq 0 ]
	[ "$output" = "$BUILT_SHA" ]
}

# validate-release.sh

@test "validate rejects a version that is not semver" {
	export VERSION=0.2.0
	run "$SCRIPTS/validate-release.sh"
	[ "$status" -eq 1 ]
	[[ "$output" == *"not a valid semver tag"* ]]
}

@test "validate rejects a missing release" {
	run "$SCRIPTS/validate-release.sh"
	[ "$status" -eq 1 ]
	[[ "$output" == *"No release found"* ]]
}

@test "validate rejects a draft" {
	release_is true true
	run "$SCRIPTS/validate-release.sh"
	[ "$status" -eq 1 ]
	[[ "$output" == *"is a draft"* ]]
}

@test "validate rejects a full release" {
	release_is false false
	tag_at "$BUILT_SHA"
	run "$SCRIPTS/validate-release.sh"
	[ "$status" -eq 1 ]
	[[ "$output" == *"is a full release"* ]]
	[ ! -s "$GITHUB_OUTPUT" ]
}

@test "validate rejects a pre-release without a tag" {
	release_is false true
	run "$SCRIPTS/validate-release.sh"
	[ "$status" -eq 1 ]
	[[ "$output" == *"Tag $VERSION does not exist"* ]]
}

@test "validate writes version and the tag commit for a pre-release" {
	release_is false true
	tag_at "$BUILT_SHA"
	run "$SCRIPTS/validate-release.sh"
	[ "$status" -eq 0 ]
	grep -qx "version=$VERSION" "$GITHUB_OUTPUT"
	grep -qx "ref=$BUILT_SHA" "$GITHUB_OUTPUT"
}

# upload-release-assets.sh

@test "upload refuses a release made a full release during the run" {
	release_is false false
	tag_at "$BUILT_SHA"
	run "$SCRIPTS/upload-release-assets.sh" "$RELEASE_DIR"
	[ "$status" -eq 1 ]
	[[ "$output" == *"no longer a pre-release"* ]]
	! grep -q "release upload" "$GH_STUB_LOG"
}

@test "upload refuses a tag moved during the run" {
	release_is false true
	tag_at "$OTHER_SHA"
	run "$SCRIPTS/upload-release-assets.sh" "$RELEASE_DIR"
	[ "$status" -eq 1 ]
	[[ "$output" == *"now points at $OTHER_SHA"* ]]
	! grep -q "release upload" "$GH_STUB_LOG"
}

@test "upload refuses a tag deleted during the run" {
	release_is false true
	run "$SCRIPTS/upload-release-assets.sh" "$RELEASE_DIR"
	[ "$status" -eq 1 ]
	! grep -q "release upload" "$GH_STUB_LOG"
}

@test "upload attaches zips, checksums and signature to the pre-release" {
	release_is false true
	tag_at "$BUILT_SHA"
	run "$SCRIPTS/upload-release-assets.sh" "$RELEASE_DIR"
	[ "$status" -eq 0 ]
	grep -q "release upload $VERSION $RELEASE_DIR/engine.zip $RELEASE_DIR/engine_SHA256SUMS $RELEASE_DIR/engine_SHA256SUMS.sig --clobber" "$GH_STUB_LOG"
}
