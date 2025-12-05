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
	Housekeeper    Housekeeper    `yaml:"housekeeper"`
}

type Housekeeper struct {
	TokenRefreshInterval        time.Duration `yaml:"tokenRefreshInterval" default:"30m"`
	TokenRefreshTriggerInterval time.Duration `yaml:"tokenRefreshTriggerInterval" default:"5m"`

	IdleSessionCleanupInterval time.Duration `yaml:"idleSessionCleanupInterval" default:"30m"`
	IdleSessionTimeout         time.Duration `yaml:"idleSessionTimeout" default:"90m"`
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
	CallbackURL                         string              `yaml:"callbackURL" default:"/sm/callback"`
	ClientAuth                          ClientAuth          `yaml:"clientAuth"`
	CSRFSecret                          commoncfg.SourceRef `yaml:"csrfSecret"`
	AdditionalQueryParametersAuthorize  []string            `yaml:"additionalQueryParametersAuthorize"`
	AdditionalQueryParametersToken      []string            `yaml:"additionalQueryParametersToken"`
	AdditionalQueryParametersIntrospect []string            `yaml:"additionalQueryParametersIntrospect"`
	AdditionalAuthContextKeys           []string            `yaml:"additionalAuthContextKeys"`
	// SessionCookieTemplate defines the template attributes for the session cookie.
	SessionCookieTemplate CookieTemplate `yaml:"sessionCookieTemplate"`
	// CSRFCookieTemplate defines the template attributes for the CSRF cookie.
	CSRFCookieTemplate CookieTemplate `yaml:"csrfCookieTemplate"`

	// Deprecated: not used anymore. Kept for a helm issue with the migrate job.
	RedirectURL string `yaml:"redirectURL" default:"/sm/redirect"`
	// Deprecated: use AdditionalQueryParametersAuthorize instead.
	AdditionalGetParametersAuthorize []string `yaml:"additionalGetParametersAuthorize"`
	// Deprecated: use AdditionalQueryParametersToken instead.
	AdditionalGetParametersToken []string `yaml:"additionalGetParametersToken"`
}

type CookieSameSiteValue string

const (
	CookieSameSiteLax    CookieSameSiteValue = "Lax"
	CookieSameSiteStrict CookieSameSiteValue = "Strict"
	CookieSameSiteNone   CookieSameSiteValue = "None"
)

type CookieTemplate struct {
	Name     string              `yaml:"name"`
	MaxAge   int                 `yaml:"maxAge"`
	Path     string              `yaml:"path"`
	Domain   string              `yaml:"domain"`
	Secure   bool                `yaml:"secure"`
	SameSite CookieSameSiteValue `yaml:"sameSite"`
	HTTPOnly bool                `yaml:"httpOnly"`
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
