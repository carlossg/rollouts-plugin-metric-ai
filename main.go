package main

import (
	"os"
	"strings"

	"github.com/argoproj-labs/rollouts-plugin-metric-ai/internal/plugin"
	rolloutsPlugin "github.com/argoproj/argo-rollouts/metricproviders/plugin/rpc"
	goPlugin "github.com/hashicorp/go-plugin"
	log "github.com/sirupsen/logrus"
)

// handshakeConfigs are used to just do a basic handshake between
// a plugin and host. If the handshake fails, a user friendly error is shown.
// This prevents users from executing bad plugins or executing a plugin
// directory. It is a UX feature, not a security feature.
var handshakeConfig = goPlugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "ARGO_ROLLOUTS_RPC_PLUGIN",
	MagicCookieValue: "metricprovider",
}

// configureLogLevel sets the log level based on environment variable
func configureLogLevel() {
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info" // default level
	}

	level, err := log.ParseLevel(strings.ToLower(logLevel))
	if err != nil {
		log.Warnf("Invalid log level '%s', using 'info' as default. Valid levels: panic, fatal, error, warn, info, debug, trace", logLevel)
		level = log.InfoLevel
	}

	log.SetLevel(level)
	log.WithField("level", level.String()).Info("Log level configured")
}

func main() {
	// Configure log level first
	configureLogLevel()

	logCtx := *log.WithFields(log.Fields{"plugin": "ai"})

	rpcPluginImp := &plugin.RpcPlugin{
		LogCtx: logCtx,
	}
	// pluginMap is the map of plugins we can dispense.
	pluginMap := map[string]goPlugin.Plugin{
		"RpcMetricProviderPlugin": &rolloutsPlugin.RpcMetricProviderPlugin{Impl: rpcPluginImp},
	}

	logCtx.Info("message from plugin", "ping", "pong")

	goPlugin.Serve(&goPlugin.ServeConfig{
		HandshakeConfig: handshakeConfig,
		Plugins:         pluginMap,
	})
}
