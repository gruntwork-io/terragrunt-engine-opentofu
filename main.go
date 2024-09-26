package main

import (
	tgengine "github.com/gruntwork-io/terragrunt-engine-go/engine"
	"github.com/gruntwork-io/terragrunt-engine-opentofu/engine"
	"github.com/hashicorp/go-hclog"
	"github.com/sirupsen/logrus"
	"os"

	"github.com/hashicorp/go-plugin"
)

const (
	engineLogLevelEnv     = "TG_ENGINE_LOG_LEVEL"
	defaultEngineLogLevel = "INFO"
)

func main() {
	engineLogLevel := os.Getenv(engineLogLevelEnv)
	if engineLogLevel == "" {
		engineLogLevel = defaultEngineLogLevel
	}
	parsedLevel, err := logrus.ParseLevel(engineLogLevel)
	if err != nil {
		logrus.Warnf("Error parsing log level: %v", err)
		parsedLevel = logrus.InfoLevel
	}
	logrus.SetLevel(parsedLevel)

	logger := hclog.NewInterceptLogger(&hclog.LoggerOptions{
		Level: hclog.LevelFromString(engineLogLevel),
	})

	plugin.Serve(&plugin.ServeConfig{
		Logger: logger,
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
