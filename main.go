package main

import (
	types "github.com/gruntwork-io/terragrunt-engine-go/types"
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
			"tofu": &types.TerragruntGRPCEngine{Impl: &engine.TofuCommandExecutor{}},
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
}
