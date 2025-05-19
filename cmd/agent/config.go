package main

import (
	"codeberg.org/n30w/jasima/pkg/chat"
	"codeberg.org/n30w/jasima/pkg/llms"
)

const (
	DefaultAgentName = ""
	// DefaultAgentConfigPath is in relation to where the binary was run and
	// not the path where the binary exists.
	DefaultAgentConfigPath        = "./cmd/configs/default_agent.toml"
	DefaultServerAddress          = "localhost:50051"
	DefaultApiUrl                 = ""
	DefaultPeers                  = ""
	DefaultInitializationFilePath = ""
	DefaultTemperatureFloat       = 1.5
	DefaultModel                  = -1
	DefaultLayer                  = -1
	DefaultDebugToggle            = false
)

type networkConfig struct {
	Router   string
	Database string
}

type userConfig struct {
	Name    string
	Peers   []string
	Layer   int32
	Model   llms.ModelConfig
	Network networkConfig
}

type config struct {
	Name          chat.Name
	Peers         []chat.Name
	Layer         chat.Layer
	ModelConfig   llms.ModelConfig
	NetworkConfig networkConfig
}
