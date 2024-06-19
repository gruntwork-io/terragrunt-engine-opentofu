package main

import (
	"plugin"
)

func main() {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: plugin.HandshakeConfig{
			ProtocolVersion:  1,
			MagicCookieKey:   "plugin",
			MagicCookieValue: "terragrunt",
		},
		Plugins: map[string]plugin.Plugin{
			"tofu": &pb.TerragruntGRPCPlugin{Impl: &TofuCommandExecutor{}},
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
}
