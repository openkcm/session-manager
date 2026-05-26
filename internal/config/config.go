// Package config defines the necessary types to configure the application.
// An example config file config.yaml is provided in the repository.
package config

import (
	"fmt"
	"reflect"
	"time"

	"github.com/creasty/defaults"
	"github.com/knadh/koanf/v2"
	"github.com/openkcm/common-sdk/pkg/commoncfg"

	sessionmanager "github.com/openkcm/session-manager"
)

type Config struct {
	commoncfg.BaseConfig `yaml:",inline"`

	HTTP HTTPServer `yaml:"http"`
	GRPC GRPCServer `yaml:"grpc"`

	Database       Database       `yaml:"database"`
	ValKey         ValKey         `yaml:"valkey"`
	Migrate        Migrate        `yaml:"migrate"`
	SessionManager SessionManager `yaml:"sessionManager"`
	Housekeeper    Housekeeper    `yaml:"housekeeper"`
	Trust          Trust          `yaml:"trust"`

	// Apps configures long-running components that satisfy the sessionmanager.App
	// interface. The map key is an operator-chosen name. Each entry MUST set
	// "module:" to the registered module ID; remaining fields are passed to the
	// module via UnmarshalExtension.
	Apps map[string]*App `yaml:"apps"`
	// AppsOrder optionally overrides the start order of apps. Apps not listed
	// here are started in parser-defined order after the listed ones. At
	// shutdown, apps are stopped in the reverse of the order in which they were
	// successfully started.
	AppsOrder []string `yaml:"appsOrder"`
}

// App is the per-entry configuration under the top-level apps: section. It
// implements sessionmanager.ExtensionConfig so it can be passed to LoadApp.
type App struct {
	Mod   string `yaml:"module"`
	koanf *koanf.Koanf
}

func (c *App) setKoanf(ko *koanf.Koanf) {
	c.koanf = ko
}

func (c *App) Module() string {
	return c.Mod
}

func (c *App) UnmarshalExtension(into sessionmanager.Module) error {
	if c.koanf == nil {
		return nil
	}
	return unmarshalExtension(into, c.koanf)
}

type Trust struct {
	Mod   string `yaml:"module" default:"trust.module.oidc"`
	koanf *koanf.Koanf
}

func (c *Trust) setKoanf(ko *koanf.Koanf) {
	c.koanf = ko
}

func (c *Trust) Module() string {
	return c.Mod
}

func (c *Trust) UnmarshalExtension(into sessionmanager.Module) error {
	return unmarshalExtension(into, c.koanf)
}

func unmarshalExtension(out any, ko *koanf.Koanf) error {
	if err := ko.UnmarshalWithConf("", out, koanfUnmarshalConf); err != nil {
		return fmt.Errorf("unmarshaling into a structure: %w", err)
	}

	setKoanf(reflect.ValueOf(out), ko)
	if err := defaults.Set(out); err != nil {
		return fmt.Errorf("setting defaults: %w", err)
	}
	return nil
}

type Housekeeper struct {
	// TriggerInterval defines how often the housekeeper jobs run.
	TriggerInterval time.Duration `yaml:"triggerInterval" default:"10m"`
	// ConcurrencyLimit defines the maximum number of sessions handled concurrently during housekeeping.
	ConcurrencyLimit int `yaml:"concurrencyLimit" default:"10"`
	// TokenRefreshTriggerInterval defines the duration before token expiry when a token refresh should be triggered.
	// This should at least match the TriggerInterval to ensure that expiring tokens are refreshed in time.
	TokenRefreshTriggerInterval time.Duration `yaml:"tokenRefreshTriggerInterval" default:"15m"`
}

type HTTPServer struct {
	Address         string        `yaml:"address" default:":8080"`
	ShutdownTimeout time.Duration `yaml:"shutdownTimeout" default:"5s"`
}

type GRPCServer struct {
	commoncfg.GRPCServer `yaml:",inline"`

	ShutdownTimeout time.Duration `yaml:"shutdownTimeout" default:"5s"`
}

type Database struct {
	Mod string `yaml:"module" default:"database.module.pgxpool"`

	koanf *koanf.Koanf
}

func (c *Database) setKoanf(ko *koanf.Koanf) {
	c.koanf = ko
}

func (c *Database) Module() string {
	return c.Mod
}

func (c *Database) UnmarshalExtension(into sessionmanager.Module) error {
	return unmarshalExtension(into, c.koanf)
}

type ValKey struct {
	Host      commoncfg.SourceRef `yaml:"host"`
	User      commoncfg.SourceRef `yaml:"user"`
	Password  commoncfg.SourceRef `yaml:"password"`
	Prefix    string              `yaml:"prefix"`
	SecretRef commoncfg.SecretRef `yaml:"secretRef"`
}

type SessionManager struct {
	IdleSessionTimeout time.Duration `yaml:"idleSessionTimeout" default:"90m"`
	SessionDuration    time.Duration `yaml:"sessionDuration" default:"12h"`

	// CallbackURL is the URL path for the OAuth2 callback endpoint, where we receive the authorization code.
	CallbackURL      string              `yaml:"callbackURL" default:"/sm/callback"`
	ClientAuth       ClientAuth          `yaml:"clientAuth"`
	CSRFSecret       commoncfg.SourceRef `yaml:"csrfSecret"`
	CSRFSecretParsed []byte              `yaml:"-"`
	// SessionCookieTemplate defines the template attributes for the session cookie.
	SessionCookieTemplate CookieTemplate `yaml:"sessionCookieTemplate"`
	// CSRFCookieTemplate defines the template attributes for the CSRF cookie.
	CSRFCookieTemplate CookieTemplate `yaml:"csrfCookieTemplate"`
	// LoginCSRFCookieTemplate defines the template attributes for the CSRF cookie.
	LoginCSRFCookieTemplate CookieTemplate `yaml:"loginCSRFCookieTemplate"`

	// AllowedRedirectBaseURLs defines the list of allowed base URLs for redirection
	// during the authorization flow and post logout. This is used to validate the redirect
	// URLs provided in the authorization request and post logout requests.
	AllowedRedirectBaseURLs []string `yaml:"allowedRedirectBaseURLs"`
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
	Mod   string `yaml:"module" default:"trust.migration.module.oidc"`
	koanf *koanf.Koanf
}

func (c *Migrate) setKoanf(ko *koanf.Koanf) {
	c.koanf = ko
}

func (c *Migrate) Module() string {
	return c.Mod
}

func (c *Migrate) UnmarshalExtension(into sessionmanager.Module) error {
	return unmarshalExtension(into, c.koanf)
}
