package config

import (
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
)

type Config struct {
	commoncfg.BaseConfig `mapstructure:",squash"`

	HTTP HTTPServer `yaml:"http"`
	GRPC GRPCServer `yaml:"grpc"`
}

type HTTPServer struct {
	Address         string        `yaml:"address" default:":8080"`
	ShutdownTimeout time.Duration `yaml:"shutdownTimeout" default:"5s"`
}

type GRPCServer struct {
	Address          string                         `yaml:"address" default:":9092"`
	ShutdownTimeout  time.Duration                  `yaml:"shutdownTimeout" default:"5s"`
	ClientAttributes commoncfg.GRPCClientAttributes `yaml:"clientAttributes"`
}
