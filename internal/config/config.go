// Package config defines the necessary types to configure the application.
// An example config file config.yaml is provided in the repository.
package config

import (
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
)

type Config struct {
	commoncfg.BaseConfig `mapstructure:",squash" yaml:",inline"`

	HTTP HTTPServer `yaml:"http"`
	GRPC GRPCServer `yaml:"grpc"`

	Database       Database       `yaml:"database"`
	ValKey         ValKey         `yaml:"valkey"`
	Migrate        Migrate        `yaml:"migrate"`
	SessionManager SessionManager `yaml:"sessionManager"`
}

type HTTPServer struct {
	Address         string        `yaml:"address" default:":8080"`
	ShutdownTimeout time.Duration `yaml:"shutdownTimeout" default:"5s"`
}

type GRPCServer struct {
	commoncfg.GRPCServer `mapstructure:",squash" yaml:",inline"`

	ShutdownTimeout time.Duration `yaml:"shutdownTimeout" default:"5s"`
}

type Database struct {
	Name     string              `yaml:"name"`
	Port     string              `yaml:"port"`
	Host     commoncfg.SourceRef `yaml:"host"`
	User     commoncfg.SourceRef `yaml:"user"`
	Password commoncfg.SourceRef `yaml:"password"`
}

type ValKey struct {
	Host     commoncfg.SourceRef `yaml:"host"`
	User     commoncfg.SourceRef `yaml:"user"`
	Password commoncfg.SourceRef `yaml:"password"`
	Prefix   string              `yaml:"prefix"`
}

type SessionManager struct {
	SessionDuration time.Duration       `yaml:"sessionDuration" default:"12h"`
	RedirectURI     string              `yaml:"redirectURI" default:"https://api.cmk/callback"`
	ClientID        commoncfg.SourceRef `yaml:"clientID"`
	CSRFSecret      string              `yaml:"csrfSecret"`
}

type Migrate struct {
	Source string `yaml:"source" default:"file://./sql"`
}
