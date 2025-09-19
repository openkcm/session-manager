package config

import (
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
)

type Config struct {
	commoncfg.BaseConfig `mapstructure:",squash"`

	HTTP       HTTPServer `yaml:"http"`
	GRPCServer GRPCServer `yaml:"grpc"`
}

type HTTPServer struct {
	Address         string        `yaml:"address" default:":8080"`
	ShutdownTimeout time.Duration `yaml:"shutdownTimeout" default:"5s"`
}

type GRPCServer struct {
	commoncfg.GRPCServer `mapstructure:",squash"`
	// also embed client attributes for the gRPC health check client
	Client commoncfg.GRPCClient
}
