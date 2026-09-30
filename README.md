# terragrunt-engine-opentofu

[Terragrunt](https://github.com/gruntwork-io/terragrunt) OpenTofu IAC engine implemented based on spec from [terragrunt-engine-go](https://github.com/gruntwork-io/terragrunt-engine-go)

## Overview

Prior to the introduction of IAC Engines, Terragrunt directly wrapped Terraform, and then OpenTofu CLI commands in order to orchestrate IAC updates in a scalable, and maintainable manner.

Over time, the Terragrunt codebase has grown in complexity, and the need for a more modular, and maintainable approach to managing IAC updates has become apparent. The OpenTofu Engine is the first Terragrunt IAC Engine implementation, and is designed to demonstrate how IAC updates can be delegated to a separate plugin that can be maintained independently of the Terragrunt codebase.

As it stands, this engine simply reproduces the existing behavior of Terragrunt, mediated by RPC calls to a plugin running locally. Note that the design of the IAC Engine system is intended to be more flexible in that it allows for myriad implementations of IAC engines, including those that may execute IAC updates in a remote environment, or include additional functionality beyond what is currently available by directly calling OpenTofu CLI commands.

We hope that this engine will inspire you to experiment and create your own IAC Engine implementations, and we look forward to seeing what you come up with!

For more information, see the [Terragrunt IAC Engine RFC](https://github.com/gruntwork-io/terragrunt/issues/3103).

## Features

### Automatic OpenTofu Binary Installation

The engine supports automatic downloading and installation of OpenTofu binaries via the convenient [tofudl library](https://github.com/opentofu/tofudl). This feature eliminates the need to manually install OpenTofu on your system and ensures consistent versions across different environments.

**Key Benefits:**

- **Version Management**: Specify exact OpenTofu versions for consistent deployments
- **Automatic Downloads**: Binaries are downloaded and cached automatically
- **Concurrent Safety**: File locking prevents race conditions during parallel downloads
- **Smart Caching**: Downloaded binaries are cached in `~/.cache/terragrunt/tofudl/` for reuse

**How it works:**

- If no version is specified, the engine uses the system's OpenTofu binary
- When a version is specified, the engine automatically downloads and caches the binary
- Subsequent runs with the same version reuse the cached binary
- File locking ensures safe concurrent access across multiple Terragrunt runs

## Usage

To utilize the OpenTofu Engine in your Terragrunt configuration, you need to specify the `engine` in HCL code.
Here's an example:

```hcl
engine {
  source  = "github.com/gruntwork-io/terragrunt-engine-opentofu"
  // Specify a fixed version if you want to pin a specific engine version instead of always
  // using the latest version of the engine.
  // version = "v0.1.0"
}
```

Pinning the version of the engine is optional, but it's recommended to do so to ensure that you're always using the same version of the engine. The latest version is on the [releases page](https://github.com/gruntwork-io/terragrunt-engine-opentofu/releases/latest). Engine versions `v0.1.0` and later require Terragrunt `v0.99.0` or later.

### Auto-Install Configuration

To enable automatic OpenTofu binary installation, you can specify the desired version and optional installation directory in your Terragrunt configuration:

```hcl
engine {
  source  = "github.com/gruntwork-io/terragrunt-engine-opentofu"

  meta = {
    tofu_version     = "v1.9.1"                # Required for auto-install: OpenTofu version to download (you can use "latest" to use the latest stable version)
    tofu_install_dir = "/custom/install/path"  # Optional: Custom installation directory
  }
}
```

**Configuration Options:**

- `tofu_version`: (Required for auto-install) The OpenTofu version to download and use.

  Supports:

  - Specific versions: `"v1.9.1"`, `"1.8.5"`
  - Latest stable: `"latest"`
  - If not specified, uses system OpenTofu binary

- `tofu_install_dir`: (Optional) Custom directory to install the binary. If not specified, uses `~/.cache/terragrunt/tofudl/bin/<version>/`

**Examples:**

```hcl
# Use latest stable OpenTofu version
engine {
  source = "github.com/gruntwork-io/terragrunt-engine-opentofu"
  meta = {
    tofu_version = "latest"
  }
}

# Use specific OpenTofu version
engine {
  source = "github.com/gruntwork-io/terragrunt-engine-opentofu"
  meta = {
    tofu_version = "v1.9.1"
  }
}

# Use specific version with custom install directory
engine {
  source = "github.com/gruntwork-io/terragrunt-engine-opentofu"
  meta = {
    tofu_version = "v1.9.1"
    tofu_install_dir = "/opt/tofu"
  }
}
```

Make sure to set the required environment variable to enable the experimental engine feature:

```bash
export TG_EXPERIMENTAL_ENGINE=1
```

## Releasing

Releases use [semantic versions](https://semver.org/) and follow the same process as [Terragrunt releases](https://terragrunt.gruntwork.io/docs/process/releases/): the release is built into a **draft**, a maintainer checks it, and publishing makes it available. Nothing reaches users before the draft is published.

Immutable releases are not enabled for this repository yet. The process does not depend on them: all files are attached before the release is published.

### Before the first release

The Release workflow needs these repository secrets, set under **Settings** > **Secrets and variables** > **Actions**:

| Secret | Purpose |
|---|---|
| `MACOS_CERTIFICATE` | Apple Developer ID certificate, P12, base64 encoded |
| `MACOS_CERTIFICATE_PASSWORD` | Password of the P12 certificate |
| `MACOS_AC_PASSWORD` | Apple notarization password |
| `MACOS_AC_PROVIDER` | Apple notarization provider (team) |
| `GW_ENGINE_GPG_KEY` | Engine GPG private key, base64 encoded, used to sign `SHA256SUMS` |
| `GW_ENGINE_GPG_KEY_PW` | Passphrase of the engine GPG key |

`GW_ENGINE_GPG_KEY` must be the key Terragrunt checks engine signatures with, fingerprint `1B73A8002338C2BB28DB30F4AF5968DA739BFC5C`. Terragrunt rejects engines signed with any other key.

### How to create a new release

1. Go to the [Releases page](https://github.com/gruntwork-io/terragrunt-engine-opentofu/releases) and click **Draft a new release**.
2. In **Choose a tag**, enter the version, for example `v0.2.0`, and select **Create new tag: v0.2.0 on publish**.
3. Set **Target** to an explicit commit SHA, not a branch name, for example the latest commit on `main`. This keeps new commits out of the release between drafting and publishing. The workflow rejects drafts that target a branch.
4. Set the release title to the version, for example `v0.2.0`.
5. Click **Save draft**.
6. Run the [Release workflow](.github/workflows/release.yml): **Actions** > **Release** > **Run workflow**, and enter the version, for example `v0.2.0`. GitHub does not start workflows for drafts, so this step is manual. The workflow reads the draft's target commit and:
   - checks that the version is a valid semantic version
   - checks that the target is a commit SHA, and that an existing tag with the same name points at that commit
   - runs the tests
   - builds binaries for all supported platforms
   - signs and notarizes the macOS binaries
   - generates `SHA256SUMS` and signs it with the engine GPG key
   - attaches the zips, `SHA256SUMS` and its signature to the draft, and checks that all of them are there
7. When the workflow succeeds, open the draft and check the attached files: seven zips, `SHA256SUMS` and `SHA256SUMS.sig`.
8. Write or finish the release notes.
9. When ready:
   - for a stable release, clear **Set as a pre-release** and check **Set as the latest release**
   - for a pre-release, check **Set as a pre-release**
10. Click **Publish release**. This creates the tag at the draft's target commit.

The same steps from the command line:

```bash
git fetch origin
gh release create v0.2.0 --draft --title v0.2.0 --target "$(git rev-parse origin/main)" --notes-file notes.md
gh workflow run release.yml -f version=v0.2.0
gh run watch "$(gh run list --workflow release.yml -L 1 --json databaseId -q '.[0].databaseId')"
gh release view v0.2.0 --json assets -q '.assets[].name'
gh release edit v0.2.0 --draft=false --latest    # for a pre-release: --draft=false --prerelease
```

If you edit the draft, for example to target a newer commit, run the Release workflow again with the same version. Editing a draft does not start a new build.

### Retrying a failed build

If the workflow fails, fix the cause, for example a missing secret, and run the Release workflow again with the same version. It rebuilds and replaces the files already on the draft. Files with other names stay on the draft; delete them by hand.

### Pre-releases

Pre-releases use semver pre-release suffixes: `-alpha.N` for early testing, `-beta.N` for broader testing, and `-rcN` for release candidates.

Terragrunt ignores pre-releases when it looks up the latest engine. Only users who set `version` in the `engine` block to the pre-release version get it.

### Testing a published release with Terragrunt

Terragrunt checks the `SHA256SUMS` signature and the engine checksum when it downloads and loads an engine. To test a published release end to end, run Terragrunt's engine tests from a checkout of [gruntwork-io/terragrunt](https://github.com/gruntwork-io/terragrunt), with the version you released:

```bash
TOFU_ENGINE_VERSION=v0.2.0 go test -tags engine -run '^TestEngine' ./test/...
```

## Contributing

Contributions are welcome! Checkout out the [Contributing Guidelines](./CONTRIBUTING.md) for more information.

## License

[Mozilla Public License v2.0](./LICENSE)
