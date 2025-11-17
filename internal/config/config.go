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
	TokenRefresher TokenRefresher `yaml:"tokenRefresher"`
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
	Host      commoncfg.SourceRef `yaml:"host"`
	User      commoncfg.SourceRef `yaml:"user"`
	Password  commoncfg.SourceRef `yaml:"password"`
	Prefix    string              `yaml:"prefix"`
	SecretRef commoncfg.SecretRef `yaml:"secretRef"`
}

type SessionManager struct {
	SessionDuration time.Duration `yaml:"sessionDuration" default:"12h"`
	// CallbackURL is the URL path for the OAuth2 callback endpoint, where we receive the authorization code.
	CallbackURL string `yaml:"callbackURL" default:"/sm/callback"`
	// RedirectURL is the URL path for redirecting after the authorization code flow finished. We can't let
	// the callback handler redirect as this needs to set cookies for the right domain.
	RedirectURL                      string              `yaml:"redirectURL" default:"/sm/redirect"`
	ClientAuth                       ClientAuth          `yaml:"clientAuth"`
	CSRFSecret                       commoncfg.SourceRef `yaml:"csrfSecret"`
	JWSSigAlgs                       []string            `yaml:"jwsSigAlgs"` // A list of supported JWT signature algorithms
	AdditionalGetParametersAuthorize []string            `yaml:"additionalGetParametersAuthorize"`
	AdditionalGetParametersToken     []string            `yaml:"additionalGetParametersToken"`
	AdditionalAuthContextKeys        []string            `yaml:"additionalAuthContextKeys"`
}

type ClientAuth struct {
	ClientID string `yaml:"clientID"`
	// Type defines how to authenticate the client.
	// Supported types are:
	//   - mtls: Mutual TLS authentication
	//   - clientSecret: Client Secret authentication
	Type string `yaml:"type" default:"mtls"`
	// MTLS contains the mTLS configuration when Type is set to "mtls".
	MTLS *commoncfg.MTLS `yaml:"mTLS"`
	// ClientSecret contains the client secret source reference when Type is set to "clientSecret".
	ClientSecret commoncfg.SourceRef `yaml:"clientSecret"`
}

type Migrate struct {
	Source string `yaml:"source" default:"file://./sql"`
}

type TokenRefresher struct {
	RefreshInterval time.Duration `yaml:"refreshInterval" default:"30m"`
}
