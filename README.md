# terragrunt-engine-opentofu

[Terragrunt](https://github.com/gruntwork-io/terragrunt) OpenTofu IAC engine implemented based on spec from [terragrunt-engine-go](https://github.com/gruntwork-io/terragrunt-engine-go)

## Overview

Prior to the introduction of IAC Engines, Terragrunt directly wrapped Terraform, and then OpenTofu CLI commands in order to orchestrate IAC updates in a scalable, and maintainable manner.

Over time, the Terragrunt codebase has grown in complexity, and the need for a more modular, and maintainable approach to managing IAC updates has become apparent. The OpenTofu Engine is the first Terragrunt IAC Engine implementation, and is designed to demonstrate how IAC updates can be delegated to a separate plugin that can be maintained independently of the Terragrunt codebase.

As it stands, this engine simply reproduces the existing behavior of Terragrunt, mediated by RPC calls to a plugin running locally. Note that the design of the IAC Engine system is intended to be more flexible in that it allows for myriad implementations of IAC engines, including those that may execute IAC updates in a remote environment, or include additional functionality beyond what is currently available by directly calling OpenTofu CLI commands.

We hope that this engine will inspire you to experiment and create your own IAC Engine implementations, and we look forward to seeing what you come up with!

For more information, see the [Terragrunt IAC Engine RFC](https://github.com/gruntwork-io/terragrunt/issues/3103).

## Automated Release Process

To initiate the release process, create a pre-release named using the following naming convention: `vx.y.z-rcdateincrement`, with the appropriate corresponding tag.
* Example Tag: `v0.0.1-rc2024053001`
  * `v0.0.1` represents the version number.
  * `-rc2024053001` indicates a release candidate, with the date and an incrementing identifier.

Workflow:
* Tag Creation:
  * Create a pre-release ending with `-rc...` to the repository.
  * This tag format will automatically trigger a CircleCI job.
* CI/CD Process:
  * CircleCI will run a build job to compile binaries and perform necessary checks.
  * Upon successful completion, a release job will be initiated.
* GitHub Release:
  * The release job creates a new GitHub release. 
  * All compiled assets, including checksums and signatures, are uploaded to the release.

## Contributing

Contributions are welcome! Checkout out the [Contributing Guidelines](./CONTRIBUTING.md) for more information.

## License

[Mozilla Public License v2.0](./LICENSE)
