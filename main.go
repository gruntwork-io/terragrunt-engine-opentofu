package main

import (
	tgengine "github.com/gruntwork-io/terragrunt-engine-go/engine"
	"github.com/gruntwork-io/terragrunt-engine-opentofu/engine"

	"github.com/hashicorp/go-plugin"
)

func main() {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: plugin.HandshakeConfig{
			ProtocolVersion:  1,
			MagicCookieKey:   "engine",
			MagicCookieValue: "terragrunt",
		},
		Plugins: map[string]plugin.Plugin{
			"tofu": &tgengine.TerragruntGRPCEngine{Impl: &engine.TofuEngine{}},
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
}
