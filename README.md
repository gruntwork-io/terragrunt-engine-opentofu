# terragrunt-engine-opentofu

OpenTofu IaC engine for [Terragrunt](https://github.com/gruntwork-io/terragrunt), built on the engine protocol from [terragrunt-engine-go](https://github.com/gruntwork-io/terragrunt-engine-go).

## Overview

Without an engine, Terragrunt runs the Terraform or OpenTofu CLI itself. An IaC engine moves that work into a separate plugin. Terragrunt starts the plugin and talks to it over RPC, so the plugin can change and ship without a Terragrunt release.

Engines are an experimental Terragrunt feature. Turn them on with `--experiment iac-engine` or `--experiment-mode`:

```bash
export TG_EXPERIMENT=iac-engine
```

This is the flagship IaC engine for Terragrunt. It runs OpenTofu on your machine and basically does what Terragrunt does without an engine.

To write your own engine, start from [terragrunt-engine-go](https://github.com/gruntwork-io/terragrunt-engine-go) and use this repository as an example. The design is in the [Terragrunt IaC engine RFC](https://github.com/gruntwork-io/terragrunt/issues/3103).

## Features

### Automatic OpenTofu install

The engine can download OpenTofu for you with the [tofudl library](https://github.com/opentofu/tofudl). You don't have to install OpenTofu yourself, and every machine runs the same version.

- Without `tofu_version`, the engine uses the `tofu` binary on your `PATH`, or the one you set with `tf_path`.
- With `tofu_version`, the engine downloads that version once and caches it in `~/.cache/terragrunt/tofudl/`. Later runs reuse the cached binary.
- A file lock stops parallel Terragrunt runs from downloading the same version at the same time.

### Shared provider cache

With OpenTofu 1.10 or newer, the engine sets `TF_PLUGIN_CACHE_DIR` to the `terragrunt/providers` directory in your user cache directory, the same one Terragrunt uses without an engine. Units in a `run --all` then share one copy of each provider, where each would otherwise download its own.

- With older OpenTofu versions the engine leaves the variable unset, because they can corrupt a cache that several runs write to at once.
- If you set `TF_PLUGIN_CACHE_DIR` yourself, the engine keeps your value.
- To turn the cache off, set `no_auto_provider_cache_dir = true` in the engine `meta`. The attribute has the same name as Terragrunt's `--no-auto-provider-cache-dir` flag.

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
    tofu_version               = "v1.9.1"                # Required for auto-install: OpenTofu version to download (you can use "latest" to use the latest stable version)
    tofu_install_dir           = "/custom/install/path"  # Optional: Custom installation directory
    no_auto_provider_cache_dir = true                    # Optional: Turn off the shared provider cache
  }
}
```

- `tofu_version` is the OpenTofu version to download, such as `"v1.9.1"` or `"1.8.5"`, or `"latest"` for the latest stable release. Without it, the engine uses the `tofu` binary on your `PATH`.
- `tofu_install_dir` is where the engine puts the binary. The default is `~/.cache/terragrunt/tofudl/bin/<version>/`.
- `tf_path` is an OpenTofu binary you installed yourself, as an absolute path or a command name on your `PATH`. You can't set it together with `tofu_version`.
- `no_auto_provider_cache_dir` turns off the [shared provider cache](#shared-provider-cache) when `true`. The default is `false`.

## Examples

### Use the latest stable version of OpenTofu

```hcl
engine {
  source = "github.com/gruntwork-io/terragrunt-engine-opentofu"
  meta = {
    tofu_version = "latest"
  }
}
```

### Use a specific version of OpenTofu

```hcl
engine {
  source = "github.com/gruntwork-io/terragrunt-engine-opentofu"
  meta = {
    tofu_version = "v1.9.1"
  }
}
```

### Use a specific version of OpenTofu with a custom install directory

```hcl
engine {
  source = "github.com/gruntwork-io/terragrunt-engine-opentofu"
  meta = {
    tofu_version = "v1.9.1"
    tofu_install_dir = "/opt/tofu"
  }
}
```

### Use an OpenTofu binary you installed yourself

```hcl
engine {
  source = "github.com/gruntwork-io/terragrunt-engine-opentofu"
  meta = {
    tf_path = "/usr/local/bin/tofu"
  }
}
```

## Contributing

Contributions are welcome. See the [contributing guidelines](./CONTRIBUTING.md).

Maintainers can find the release process in [docs/releasing.md](./docs/releasing.md).

## License

[Mozilla Public License v2.0](./LICENSE)
