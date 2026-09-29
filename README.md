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
  // version = "v0.0.6"
}
```

Pinning the version of the engine is optional, but it's recommended to do so to ensure that you're always using the same version of the engine.

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

## Release Process

Publishing a pre-release starts the release automatically. Terragrunt ignores pre-releases when it looks for the latest engine, so users get the new version only after it is made the latest release.

1. **Publish a pre-release.**
   - In GitHub, open **Releases** > **Draft a new release**.
   - Create a new tag named after the version, for example `v0.2.0`, on `main`.
   - Write the release notes.
   - Check **Set as a pre-release** and click **Publish release**.

   Or from the command line:

   ```bash
   gh release create v0.2.0 --prerelease --title v0.2.0 --target main --notes-file notes.md
   ```

   Saving a draft does not start anything, because GitHub does not run workflows for drafts.

2. **Wait for the [Release](.github/workflows/release.yml) workflow.** It starts automatically and:
   - runs the tests on the commit the tag points at
   - builds binaries for all supported platforms from that commit
   - signs and notarizes the macOS binaries
   - attaches the zips, `SHA256SUMS` and its GPG signature to the pre-release

   If it fails, for example because a secret is missing, fix the problem, open the failed run and click **Re-run all jobs**. Files with the same names are replaced.

3. **Make it the latest release.** Check the attached files and the release notes, then edit the release, clear **Set as a pre-release** and save.

The Release workflow needs these repository secrets:

| Secret | Purpose |
|---|---|
| `MACOS_CERTIFICATE` | Apple Developer ID certificate, P12, base64 encoded |
| `MACOS_CERTIFICATE_PASSWORD` | Password of the P12 certificate |
| `MACOS_AC_PASSWORD` | Apple notarization password |
| `MACOS_AC_PROVIDER` | Apple notarization provider (team) |
| `GW_ENGINE_GPG_KEY` | Engine GPG private key, base64 encoded, used to sign `SHA256SUMS` |
| `GW_ENGINE_GPG_KEY_PW` | Passphrase of the engine GPG key |

## Contributing

Contributions are welcome! Checkout out the [Contributing Guidelines](./CONTRIBUTING.md) for more information.

## License

[Mozilla Public License v2.0](./LICENSE)
