package main

import (
	"codeberg.org/n30w/jasima/chat"
	"codeberg.org/n30w/jasima/llms"
	"codeberg.org/n30w/jasima/memory"
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

type channels struct {
	responses memory.MessageChannel
	llm       memory.MessageChannel
}
