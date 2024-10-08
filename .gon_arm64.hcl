# See https://github.com/gruntwork-io/terraform-aws-ci/blob/main/modules/sign-binary-helpers/
# for further instructions on how to sign the binary + submitting for notarization.

source = ["./bin/terragrunt-iac-engine-opentofu_darwin_arm64"]

bundle_id = "io.gruntwork.app.terragrunt"

apple_id {
  username = "machine.apple@gruntwork.io"
}

sign {
  application_identity = "Developer ID Application: Gruntwork, Inc."
}

zip {
  output_path = "terragrunt-iac-engine-opentofu_arm64.zip"
}
