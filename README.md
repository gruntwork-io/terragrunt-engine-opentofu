# terragrunt-engine-opentofu

OpenTofu Terragrunt engine implemented based on spec from [terragrunt-engine-go](https://github.com/gruntwork-io/terragrunt-engine-go)
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

## License

[Mozilla Public License v2.0](./LICENSE)

