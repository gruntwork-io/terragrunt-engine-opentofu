# terragrunt-engine-opentofu

OpenTofu IaC engine for [Terragrunt](https://github.com/gruntwork-io/terragrunt), built on the engine protocol from [terragrunt-engine-go](https://github.com/gruntwork-io/terragrunt-engine-go).

## Overview

Without an engine, Terragrunt runs the Terraform or OpenTofu CLI itself. An IaC engine moves that work into a separate plugin. Terragrunt starts the plugin and talks to it over RPC, so the plugin can change and ship without a Terragrunt release.

This is the first engine. It runs OpenTofu on your machine and does what Terragrunt does without an engine. Other engines can do more, for example run OpenTofu on a remote machine.

To write your own engine, start from [terragrunt-engine-go](https://github.com/gruntwork-io/terragrunt-engine-go) and use this repository as an example. The design is in the [Terragrunt IaC engine RFC](https://github.com/gruntwork-io/terragrunt/issues/3103).

## Features

### Automatic OpenTofu install

The engine can download OpenTofu for you with the [tofudl library](https://github.com/opentofu/tofudl). You don't have to install OpenTofu yourself, and every machine runs the same version.

- Without `tofu_version`, the engine uses the `tofu` binary on your `PATH`.
- With `tofu_version`, the engine downloads that version once and caches it in `~/.cache/terragrunt/tofudl/`. Later runs reuse the cached binary.
- A file lock stops parallel Terragrunt runs from downloading the same version at the same time.

## Usage

Add an `engine` block to your Terragrunt configuration:

```hcl
engine {
  source  = "github.com/gruntwork-io/terragrunt-engine-opentofu"
  // Specify a fixed version if you want to pin a specific engine version instead of always
  // using the latest version of the engine.
  // version = "v0.1.0"
}
```

Without `version`, Terragrunt uses the latest engine release. Pin a version so every run uses the same engine. The latest version is on the [releases page](https://github.com/gruntwork-io/terragrunt-engine-opentofu/releases/latest). Engine `v0.1.0` and later need Terragrunt `v0.99.0` or later.

### Auto-install configuration

To have the engine download OpenTofu, set the version in `meta`, and optionally the install directory:

```hcl
engine {
  source  = "github.com/gruntwork-io/terragrunt-engine-opentofu"

  meta = {
    tofu_version     = "v1.9.1"                # Required for auto-install: OpenTofu version to download (you can use "latest" to use the latest stable version)
    tofu_install_dir = "/custom/install/path"  # Optional: Custom installation directory
  }
}
```

- `tofu_version` is the OpenTofu version to download, such as `"v1.9.1"` or `"1.8.5"`, or `"latest"` for the latest stable release. Without it, the engine uses the `tofu` binary on your `PATH`.
- `tofu_install_dir` is where the engine puts the binary. The default is `~/.cache/terragrunt/tofudl/bin/<version>/`.

Examples:

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

Engines are an experimental Terragrunt feature. Turn them on with this environment variable, or run Terragrunt with `--experiment iac-engine`:

```bash
export TG_EXPERIMENTAL_ENGINE=1
```

## Releasing

Releases use [semantic versions](https://semver.org/) and the same process as [Terragrunt releases](https://terragrunt.gruntwork.io/docs/process/releases/). The Release workflow builds the release into a draft, a maintainer checks the draft, and publishing it makes the release available. Users get nothing before you publish.

Immutable releases are not enabled for this repository yet. The process doesn't need them, because the workflow attaches all files before you publish.

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

1. Go to the [releases page](https://github.com/gruntwork-io/terragrunt-engine-opentofu/releases) and click **Draft a new release**.
2. In **Choose a tag**, enter the version, for example `v0.2.0`, and select **Create new tag: v0.2.0 on publish**.
3. Set **Target** to a commit SHA, not a branch name. Use the latest commit on `main`, for example. A SHA keeps new commits out of the release between drafting and publishing, and the workflow rejects drafts that target a branch.
4. Set the release title to the version, for example `v0.2.0`.
5. Click **Save draft**.
6. Run the [Release workflow](.github/workflows/release.yml) from **Actions** > **Release** > **Run workflow**, and enter the version. GitHub doesn't start workflows for drafts, so you start this one by hand. The run shows up as `Release v0.2.0`. It builds the draft's target commit and:
   - checks that the version is a valid semantic version
   - checks that the target is a commit SHA, and that an existing tag with the same name points at that commit
   - runs the tests
   - builds binaries for all supported platforms
   - signs and notarizes the macOS binaries
   - generates `SHA256SUMS` and signs it with the engine GPG key
   - attaches the zips, `SHA256SUMS` and its signature to the draft, then checks that all of them are there
7. When the workflow succeeds, open the draft and check the files. There should be seven zips, `SHA256SUMS` and `SHA256SUMS.sig`.
8. Write or finish the release notes.
9. For a stable release, clear **Set as a pre-release** and check **Set as the latest release**. For a pre-release, check **Set as a pre-release**.
10. Click **Publish release**. Publishing creates the tag at the draft's target commit.

The same steps from the command line. First create the draft and build it:

```bash
git fetch origin
gh release create v0.2.0 --draft --title v0.2.0 --target "$(git rev-parse origin/main)" --notes-file notes.md
gh workflow run release.yml -f version=v0.2.0
gh run watch --exit-status    # pick "Release v0.2.0"; exits non-zero if the run fails
```

Publish only after the run succeeded and the files look right:

```bash
gh release view v0.2.0 --json assets -q '.assets[].name'
gh release edit v0.2.0 --draft=false --latest    # for a pre-release: --draft=false --prerelease
```

Editing a draft doesn't start a new build. If you change the draft, for example to target a newer commit, run the Release workflow again with the same version.

### Retrying a failed build

If the workflow fails, fix the cause, for example a missing secret, and run the Release workflow again with the same version. It rebuilds and replaces the files on the draft. Files with other names stay, so delete those by hand.

### Pre-releases

Pre-release versions end in `-alpha.N` for early testing, `-beta.N` for broader testing, or `-rcN` for release candidates.

Terragrunt skips pre-releases when it looks up the latest engine. Only users who set `version` in the `engine` block to the pre-release version get it.

### Testing a published release with Terragrunt

Terragrunt checks the `SHA256SUMS` signature and the engine checksum when it downloads and loads an engine. To test a published release end to end, run Terragrunt's engine tests with the version you released, from a checkout of [gruntwork-io/terragrunt](https://github.com/gruntwork-io/terragrunt):

```bash
TOFU_ENGINE_VERSION=v0.2.0 go test -tags engine -run '^TestEngine' ./test/...
```

## Contributing

Contributions are welcome. See the [contributing guidelines](./CONTRIBUTING.md).

## License

[Mozilla Public License v2.0](./LICENSE)
