package network

import (
	"net"
)

type ServerBase struct {
	config   *config
	errs     chan<- error
	listener net.Listener
}

func newServerBase(cfg *config, errs chan<- error) (*ServerBase, error) {
	lis, err := net.Listen(cfg.protocol, cfg.addr)
	if err != nil {
		return nil, err
	}

	b := &ServerBase{
		config:   cfg,
		errs:     errs,
		listener: lis,
	}

	return b, nil
}

const (
	defaultServerHost    = "localhost"
	defaultProtocol      = "tcp"
	defaultGRPCProtocol  = "tcp"
	defaultGRPCPort      = "50051"
	defaultWebServerPort = "7070"
)

var (
	defaultChatServerConfig = &config{
		host:     defaultServerHost,
		port:     defaultGRPCPort,
		protocol: defaultGRPCProtocol,
	}

	defaultWebServerConfig = &config{
		host:     defaultServerHost,
		port:     defaultWebServerPort,
		protocol: defaultProtocol,
	}
)

type config struct {
	addr     string
	host     string
	port     string
	protocol string
}

func newConfigWithOpts(cfg *config, opts ...func(*config)) *config {
	for _, opt := range opts {
		opt(cfg)
	}

	if cfg.addr == "" {
		cfg.addr = net.JoinHostPort(cfg.host, cfg.port)
	}

	newConf := &config{
		addr:     cfg.addr,
		host:     cfg.host,
		port:     cfg.port,
		protocol: cfg.protocol,
	}

	return newConf
}

func WithPort(port string) func(*config) {
	return func(cfg *config) {
		cfg.port = port
	}
}
