//go:build helmtests

package main_test

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/openkcm/common-sdk/pkg/commoncfg"

	sessionmanager "github.com/openkcm/session-manager"
	"github.com/openkcm/session-manager/internal/config"
	"github.com/openkcm/session-manager/modules/credentials/oauth2"
	smpgxpool "github.com/openkcm/session-manager/modules/database/pgxpool"
	valkeymodule "github.com/openkcm/session-manager/modules/sessionstore/valkey"
)

// The tests in this file check the files that the configuration refers to, as values.yaml
// documents them: the example contents of the files, read with the value reference loader
// that Session Manager uses (common-sdk), and the PEM files of the mTLS settings and of the
// PostgreSQL client, with certificates and keys generated at test time.

// credentialsDir is the directory of the files in the examples of values.yaml.
const credentialsDir = "/etc/credentials/"

// documentedSettings are the values with every documented setting and the documented
// alternatives (see configload_test.go). Together they contain every file reference of the
// examples in values.yaml.
var documentedSettings = []configInput{
	{name: "all documented settings", args: []string{"-f", "testdata/values-all-config.yaml"}},
	{
		name: "documented alternatives",
		args: []string{"-f", "testdata/values-config-alternatives.yaml"},
	},
}

// TestDocumentedFileContents writes the example content of every file that values.yaml
// documents ("Example content of <path>:") to a temporary directory and reads it through the
// documented value references to the file, as Session Manager does at startup. Every
// documented example content must be referred to by a documented setting.
func TestDocumentedFileContents(t *testing.T) {
	documented := documentedFileContents(t)

	// The values that the documented JSON examples hold at the documented jsonPath.
	jsonValues := map[string]string{
		"/etc/credentials/client-auth.json $.secret": "<client secret>",
		"/etc/credentials/db.json $.password":        "<password>",
	}

	used := make(map[string]bool)
	for _, input := range documentedSettings {
		t.Run(input.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, section := range configSections(t, loadRenderedConfig(t, input.args...)) {
				for _, ref := range fileRefs(section) {
					content, ok := documented[ref.File.Path]
					if !ok {
						continue
					}
					used[ref.File.Path] = true
					checkPEMBlocks(t, ref.File.Path, content)

					want := content
					if ref.File.Format == commoncfg.JSONFileFormat && ref.File.JSONPath != "" {
						key := ref.File.Path + " " + ref.File.JSONPath
						if want, ok = jsonValues[key]; !ok {
							t.Errorf("no expected value for %s", key)
							continue
						}
					}
					local := *ref
					local.File.Path = writeCredentialFile(t, dir, ref.File.Path, []byte(content))
					got, err := commoncfg.LoadValueFromSourceRef(local)
					if err != nil {
						t.Errorf("reading the example content of %s with %+v: %v",
							ref.File.Path, ref.File, err)
						continue
					}
					if string(got) != want {
						t.Errorf("value of %s with %+v: got %q, want %q",
							ref.File.Path, ref.File, got, want)
					}
				}
			}
		})
	}
	for documentedPath := range documented {
		if !used[documentedPath] {
			t.Errorf("values.yaml shows the content of %s, but no documented setting refers to it",
				documentedPath)
		}
	}
}

// TestValueFileFormats checks the formats of value reference files that values.yaml
// describes under "Files of value references".
func TestValueFileFormats(t *testing.T) {
	dir := t.TempDir()
	read := func(t *testing.T, content string, format commoncfg.FileFormat, jsonPath string,
	) (string, error) {
		t.Helper()
		file := filepath.Join(dir, strings.ReplaceAll(t.Name(), "/", "_"))
		if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		value, err := commoncfg.LoadValueFromSourceRef(commoncfg.SourceRef{
			Source: commoncfg.FileSourceValue,
			File:   commoncfg.CredentialFile{Path: file, Format: format, JSONPath: jsonPath},
		})
		return string(value), err
	}
	const flat = `{ "user": "session_manager", "password": "<password>" }`
	const nested = `{ "postgres": { "user": "session_manager", "password": "<password>" } }`

	for _, tt := range []struct {
		name, content string
		format        commoncfg.FileFormat
		jsonPath      string
		want          string
	}{
		{name: "binary: the whole file", content: "<password>",
			format: commoncfg.BinaryFileFormat, want: "<password>"},
		{name: "binary: a trailing newline is part of the value", content: "<password>\n",
			format: commoncfg.BinaryFileFormat, want: "<password>\n"},
		{name: "json: a key", content: flat,
			format: commoncfg.JSONFileFormat, jsonPath: "$.password", want: "<password>"},
		{name: "json: a key of a nested object", content: nested,
			format: commoncfg.JSONFileFormat, jsonPath: "$.postgres.password", want: "<password>"},
		{name: "json: a key with a hyphen", content: `{ "client-secret": "<client secret>" }`,
			format: commoncfg.JSONFileFormat, jsonPath: "$.client-secret", want: "<client secret>"},
		{name: "json: without jsonPath, the whole file", content: nested,
			format: commoncfg.JSONFileFormat, want: nested},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := read(t, tt.content, tt.format, tt.jsonPath)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}

	for _, tt := range []struct{ name, content, jsonPath string }{
		{name: "json: the value must be a string", content: `{ "port": 5432 }`, jsonPath: "$.port"},
		{name: "json: the key must exist", content: flat, jsonPath: "$.secret"},
		{name: "json: the bracket form is not supported", content: flat,
			jsonPath: "$['password']"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := read(t, tt.content, commoncfg.JSONFileFormat, tt.jsonPath); err == nil {
				t.Errorf("no error, value %q", got)
			}
		})
	}
}

// TestDocumentedMTLSFiles loads every documented mTLS setting with files in the documented
// formats, as the modules and common-sdk do at startup (commoncfg.LoadMTLSConfig), and
// connects to a TLS server that requires a client certificate issued by the client root
// CA. The client certificate file holds the certificate and its intermediate CA; the
// serverCa file holds two CA certificates.
func TestDocumentedMTLSFiles(t *testing.T) {
	pki := newTestPKI(t)
	cfg := loadRenderedConfig(t, "-f", "testdata/values-all-config.yaml")
	store := new(valkeymodule.Module)
	if err := cfg.ValKey.UnmarshalExtension(store); err != nil {
		t.Fatal(err)
	}
	settings := []struct {
		name string
		mtls *commoncfg.MTLS
	}{
		{name: "valkey.secretRef.mtls", mtls: &store.SecretRef.MTLS},
		{name: "sessionManager.clientAuth.mTLS", mtls: cfg.SessionManager.ClientAuth.MTLS},
		{name: "audit.httpClient.mtls", mtls: cfg.Audit.HTTPClient.MTLS},
		{name: "telemetry.logs.secretRef.mtls", mtls: &cfg.Telemetry.Logs.SecretRef.MTLS},
	}

	for _, setting := range settings {
		for _, key := range pki.clientKeys {
			t.Run(setting.name+"/"+key.name, func(t *testing.T) {
				if setting.mtls == nil {
					t.Fatal("not set in testdata/values-all-config.yaml")
				}
				local := writeMTLSFiles(t, t.TempDir(), setting.mtls, pki.files(key))
				tlsConfig, err := commoncfg.LoadMTLSConfig(local)
				if err != nil {
					t.Fatalf("loading the mTLS settings: %v", err)
				}
				if tlsConfig.RootCAs == nil {
					// The example has neither serverCa nor rootCAs: Session Manager trusts the
					// CA certificates of the image, and the server CA of the test is none of them.
					tlsConfig.RootCAs = pki.serverCAPool
				}
				if err := pki.handshake(t, tlsConfig, tlsConfig.ServerName); err != nil {
					t.Fatal(err)
				}
			})
		}
	}

	valkeyMTLS := settings[0].mtls
	t.Run("a key that does not match the certificate fails", func(t *testing.T) {
		files := pki.files(pki.clientKeys[0])
		files.key = pki.otherKeyPEM
		local := writeMTLSFiles(t, t.TempDir(), valkeyMTLS, files)
		if _, err := commoncfg.LoadMTLSConfig(local); err == nil {
			t.Error("no error")
		}
	})
	t.Run("a CA file without a certificate trusts no server", func(t *testing.T) {
		files := pki.files(pki.clientKeys[0])
		files.serverCA = pki.otherKeyPEM
		local := writeMTLSFiles(t, t.TempDir(), valkeyMTLS, files)
		tlsConfig, err := commoncfg.LoadMTLSConfig(local)
		if err != nil {
			t.Fatalf("loading the mTLS settings: %v", err)
		}
		err = pki.handshake(t, tlsConfig, tlsConfig.ServerName)
		if !errors.As(err, new(x509.UnknownAuthorityError)) {
			t.Errorf("got %v, want an unknown authority error", err)
		}
	})
}

// TestClientAuthMTLSFiles provisions Session Manager's credentials module with the
// documented `clientAuth` of type mtls and files in the documented formats, and fetches
// the OpenID configuration of an identity provider that requires the client certificate.
func TestClientAuthMTLSFiles(t *testing.T) {
	pki := newTestPKI(t)
	cfg := loadRenderedConfig(t, "-f", "testdata/values-all-config.yaml")
	clientAuth := &cfg.SessionManager.ClientAuth
	if clientAuth.Type != "mtls" || clientAuth.MTLS == nil || clientAuth.MTLS.Attributes == nil {
		t.Fatalf("testdata/values-all-config.yaml has no clientAuth of type mtls: %+v", clientAuth)
	}
	clientAuth.MTLS = writeMTLSFiles(t, t.TempDir(), clientAuth.MTLS, pki.files(pki.clientKeys[0]))

	clientNames := make(chan string, 10)
	var idp *httptest.Server
	discoveryHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(w, r)
			return
		}
		clientNames <- r.TLS.PeerCertificates[0].Subject.CommonName
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"issuer": idp.URL}); err != nil {
			t.Errorf("writing the OpenID configuration: %v", err)
		}
	}
	idp = httptest.NewUnstartedServer(http.HandlerFunc(discoveryHandler))
	idp.TLS = pki.serverTLSConfig(t, clientAuth.MTLS.Attributes.ServerName)
	idp.StartTLS()
	defer idp.Close()

	ctx, cancel := sessionmanager.NewContext(t.Context())
	defer cancel(nil)
	ctx = config.WithContext(ctx, cfg)
	module := new(oauth2.Module)
	if err := module.Provision(ctx); err != nil {
		t.Fatalf("provisioning the credentials module: %v", err)
	}
	discovery, err := module.GetDiscoveryConfig(ctx, idp.URL)
	if err != nil {
		t.Fatalf("fetching the OpenID configuration over mTLS: %v", err)
	}
	if discovery.Issuer != idp.URL {
		t.Errorf("issuer %q, want %q", discovery.Issuer, idp.URL)
	}
	if name := <-clientNames; name != clientCommonName {
		t.Errorf("client certificate %q, want %q", name, clientCommonName)
	}
}

// TestPostgresTLSFiles parses the connection settings as the database module does, with the
// PGSSL* environment variables and files in the formats that values.yaml documents at
// `config.database`, and connects to a TLS server with the resulting TLS configuration.
func TestPostgresTLSFiles(t *testing.T) {
	pki := newTestPKI(t)
	const host = "postgresql.services.svc.cluster.local"
	// The connection string that modules/database/pgxpool builds.
	const connString = "host=" + host +
		" user=session_manager password=x dbname=session_manager port=5432"

	setFiles := func(t *testing.T, files mtlsFiles) {
		t.Helper()
		dir := t.TempDir()
		for variable, file := range map[string]struct {
			path    string
			content []byte
		}{
			"PGSSLROOTCERT": {path: "database/ca.crt", content: files.serverCA},
			"PGSSLCERT":     {path: "database/tls.crt", content: files.cert},
			"PGSSLKEY":      {path: "database/tls.key", content: files.key},
		} {
			t.Setenv(variable, writeCredentialFile(t, dir, credentialsDir+file.path, file.content))
		}
		t.Setenv("PGSSLMODE", "verify-full")
	}

	for _, key := range pki.clientKeys {
		t.Run(key.name, func(t *testing.T) {
			setFiles(t, pki.files(key))
			poolConfig, err := pgxpool.ParseConfig(connString)
			if err != nil {
				t.Fatalf("parsing the connection settings: %v", err)
			}
			tlsConfig := poolConfig.ConnConfig.TLSConfig
			if tlsConfig == nil || tlsConfig.ServerName != host ||
				len(tlsConfig.Certificates) != 1 {
				t.Fatalf("not verify-full with a client certificate: %+v", tlsConfig)
			}
			if err := pki.handshake(t, tlsConfig, host); err != nil {
				t.Fatal(err)
			}
		})
	}

	t.Run("a CA file without a certificate fails", func(t *testing.T) {
		files := pki.files(pki.clientKeys[0])
		files.serverCA = pki.otherKeyPEM
		setFiles(t, files)
		if _, err := pgxpool.ParseConfig(connString); err == nil {
			t.Error("no error")
		}
	})
}

// documentedFileContents returns the example contents of files in values.yaml, by path: the
// comment lines below a comment that contains "Example content of <path>" and ends with a
// colon, on the same or a following line, indented by two more spaces than that comment.
func documentedFileContents(t *testing.T) map[string]string {
	t.Helper()
	data, err := os.ReadFile(path + "/values.yaml")
	if err != nil {
		t.Fatal(err)
	}
	marker := regexp.MustCompile(`Example content of (/[^\s:,()]+)`)
	indentOf := func(text string) int { return len(text) - len(strings.TrimLeft(text, " ")) }

	lines := strings.Split(string(data), "\n")
	contents := make(map[string]string)
	for i := 0; i < len(lines); i++ {
		text, ok := commentText(lines[i])
		match := marker.FindStringSubmatch(text)
		if !ok || match == nil {
			continue
		}
		filePath, indent := match[1], indentOf(text)
		for !strings.HasSuffix(strings.TrimSpace(text), ":") {
			i++
			if i == len(lines) {
				t.Fatalf("values.yaml: the description of the content of %s does not end", filePath)
			}
			if text, ok = commentText(lines[i]); !ok {
				t.Fatalf("values.yaml:%d: the description of the content of %s does not end "+
					"with a colon", i+1, filePath)
			}
		}
		var content []string
		for i+1 < len(lines) {
			next, ok := commentText(lines[i+1])
			if !ok || strings.TrimSpace(next) == "" || indentOf(next) < indent+2 {
				break
			}
			content = append(content, next[indent+2:])
			i++
		}
		if len(content) == 0 {
			t.Errorf("values.yaml:%d: no example content of %s", i+1, filePath)
		}
		if _, ok := contents[filePath]; ok {
			t.Errorf("values.yaml:%d: a second example content of %s", i+1, filePath)
		}
		contents[filePath] = strings.Join(content, "\n")
	}
	if len(contents) == 0 {
		t.Fatal("values.yaml shows the content of no file")
	}
	return contents
}

// commentText returns the text after the "#" of a comment line.
func commentText(line string) (string, bool) {
	return strings.CutPrefix(strings.TrimLeft(line, " "), "#")
}

// checkPEMBlocks checks the documented content of a PEM file: certificates (.crt) or one
// private key (.key), in blocks of the types that the reading code accepts.
func checkPEMBlocks(t *testing.T, filePath, content string) {
	t.Helper()
	var allowed []string
	switch filepath.Ext(filePath) {
	case ".crt":
		allowed = []string{"CERTIFICATE"}
	case ".key":
		allowed = []string{"PRIVATE KEY", "RSA PRIVATE KEY", "EC PRIVATE KEY"}
	default:
		return
	}
	var blocks []string
	open := ""
	for line := range strings.SplitSeq(content, "\n") {
		begin, isBegin := strings.CutPrefix(line, "-----BEGIN ")
		switch {
		case isBegin && open == "":
			open = strings.TrimSuffix(begin, "-----")
			blocks = append(blocks, open)
		case open != "" && line == "-----END "+open+"-----":
			open = ""
		case open == "" || strings.TrimSpace(line) == "":
			t.Errorf("example content of %s: unexpected line %q", filePath, line)
		}
	}
	if open != "" || len(blocks) == 0 || (filepath.Ext(filePath) == ".key" && len(blocks) != 1) {
		t.Errorf("example content of %s: unexpected blocks %q", filePath, blocks)
	}
	for _, block := range blocks {
		if !slices.Contains(allowed, block) {
			t.Errorf("example content of %s: block type %q, want one of %q",
				filePath, block, allowed)
		}
	}
}

// loadRenderedConfig renders the chart with args and loads the configuration as Session
// Manager does at startup.
func loadRenderedConfig(t *testing.T, args ...string) *config.Config {
	t.Helper()
	configMap, _ := configMaps(t, renderManifests(t, args...))
	dir := t.TempDir()
	configYAML := []byte(configMap.Data["config.yaml"])
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), configYAML, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load("{}", dir)
	if err != nil {
		t.Fatalf("loading config.yaml: %v", err)
	}
	return cfg
}

// configSections returns the configuration and the module sections with value references,
// database and valkey, decoded by their modules.
func configSections(t *testing.T, cfg *config.Config) []any {
	t.Helper()
	database := new(smpgxpool.PostgresModule)
	if err := cfg.Database.UnmarshalExtension(database); err != nil {
		t.Fatal(err)
	}
	store := new(valkeymodule.Module)
	if err := cfg.ValKey.UnmarshalExtension(store); err != nil {
		t.Fatal(err)
	}
	return []any{cfg, database, store}
}

// fileRefs returns the value references with `source: file` in v, a pointer to a struct.
func fileRefs(v any) []*commoncfg.SourceRef {
	sourceRefType := reflect.TypeFor[commoncfg.SourceRef]()
	var refs []*commoncfg.SourceRef
	var walk func(v reflect.Value)
	walk = func(v reflect.Value) {
		switch v.Kind() {
		case reflect.Pointer, reflect.Interface:
			if !v.IsNil() {
				walk(v.Elem())
			}
		case reflect.Struct:
			if v.Type() == sourceRefType {
				ref, ok := reflect.TypeAssert[*commoncfg.SourceRef](v.Addr())
				if ok && ref.Source == commoncfg.FileSourceValue {
					refs = append(refs, ref)
				}
				return
			}
			for i := range v.NumField() {
				if v.Type().Field(i).IsExported() {
					walk(v.Field(i))
				}
			}
		case reflect.Slice:
			for i := range v.Len() {
				walk(v.Index(i))
			}
		case reflect.Map:
			for _, key := range v.MapKeys() {
				walk(v.MapIndex(key))
			}
		default:
		}
	}
	walk(reflect.ValueOf(v))
	return refs
}

// writeCredentialFile writes content to the place in dir of documentedPath, a path below
// /etc/credentials, and returns that place.
func writeCredentialFile(t *testing.T, dir, documentedPath string, content []byte) string {
	t.Helper()
	relative, ok := strings.CutPrefix(documentedPath, credentialsDir)
	if !ok {
		t.Fatalf("%s is not below %s", documentedPath, credentialsDir)
	}
	local := filepath.Join(dir, relative)
	if err := os.MkdirAll(filepath.Dir(local), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(local, content, 0o600); err != nil {
		t.Fatal(err)
	}
	return local
}

// mtlsFiles is the content of the files of an mTLS setting.
type mtlsFiles struct {
	cert, key, serverCA, rootCA []byte
}

// writeMTLSFiles writes files to the places in dir of the files that the documented mTLS
// setting refers to, and returns a copy of the setting that refers to them.
func writeMTLSFiles(t *testing.T, dir string, documented *commoncfg.MTLS, files mtlsFiles,
) *commoncfg.MTLS {
	t.Helper()
	write := func(ref commoncfg.SourceRef, content []byte) commoncfg.SourceRef {
		if ref.Source != commoncfg.FileSourceValue ||
			ref.File.Format != commoncfg.BinaryFileFormat {
			t.Fatalf("not a file with format binary: %+v", ref)
		}
		ref.File.Path = writeCredentialFile(t, dir, ref.File.Path, content)
		return ref
	}
	local := *documented
	local.Cert = write(documented.Cert, files.cert)
	local.CertKey = write(documented.CertKey, files.key)
	if documented.ServerCA != nil {
		serverCA := write(*documented.ServerCA, files.serverCA)
		local.ServerCA = &serverCA
	}
	local.RootCAs = nil
	for _, rootCA := range documented.RootCAs {
		local.RootCAs = append(local.RootCAs, write(rootCA, files.rootCA))
	}
	return &local
}

// clientCommonName is the common name of the client certificates of the tests.
const clientCommonName = "session-manager"

// testPKI holds the certificates and keys of a test, all generated at test time: client
// certificates issued by an intermediate CA of the client root CA, and server certificates
// issued by the server CA.
type testPKI struct {
	clientKeys   []clientKey
	clientRoots  *x509.CertPool
	serverCA     *x509.Certificate
	serverCAKey  crypto.Signer
	serverCAPEM  []byte
	serverCAPool *x509.CertPool
	otherCAPEM   []byte // a CA certificate that issued none of the certificates
	otherKeyPEM  []byte // a private key of none of the certificates
}

// clientKey is a client certificate and its private key in one of the documented
// encodings.
type clientKey struct {
	name  string
	chain []byte // the client certificate and the intermediate CA certificate, PEM
	key   []byte // the private key, PEM
}

// newTestPKI generates the certificates and keys of a test: client certificates for an
// ECDSA and an RSA key, with each key in each documented encoding (PKCS #8, SEC 1, PKCS #1).
func newTestPKI(t *testing.T) *testPKI {
	t.Helper()
	rootKey, intermediateKey, serverCAKey := ecdsaKey(t), ecdsaKey(t), ecdsaKey(t)
	root := newCertificate(t, "client root CA", true, nil, rootKey, nil, nil)
	intermediate := newCertificate(t, "client intermediate CA", true, nil, intermediateKey,
		root, rootKey)
	serverCA := newCertificate(t, "server CA", true, nil, serverCAKey, nil, nil)
	otherKey := ecdsaKey(t)
	otherCA := newCertificate(t, "other CA", true, nil, otherKey, nil, nil)

	pki := &testPKI{
		clientRoots:  x509.NewCertPool(),
		serverCA:     serverCA,
		serverCAKey:  serverCAKey,
		serverCAPEM:  pemCertificate(serverCA),
		serverCAPool: x509.NewCertPool(),
		otherCAPEM:   pemCertificate(otherCA),
		otherKeyPEM:  pemPrivateKey(t, "PRIVATE KEY", otherKey),
	}
	pki.clientRoots.AddCert(root)
	pki.serverCAPool.AddCert(serverCA)

	chain := func(key crypto.Signer) []byte {
		leaf := newCertificate(t, clientCommonName, false, nil, key, intermediate, intermediateKey)
		return append(pemCertificate(leaf), pemCertificate(intermediate)...)
	}
	ecKey, rsaKey := ecdsaKey(t), rsaTestKey(t)
	ecChain, rsaChain := chain(ecKey), chain(rsaKey)
	pki.clientKeys = []clientKey{
		{name: "ECDSA key, PKCS #8", chain: ecChain, key: pemPrivateKey(t, "PRIVATE KEY", ecKey)},
		{name: "ECDSA key, SEC 1", chain: ecChain, key: pemPrivateKey(t, "EC PRIVATE KEY", ecKey)},
		{name: "RSA key, PKCS #8", chain: rsaChain, key: pemPrivateKey(t, "PRIVATE KEY", rsaKey)},
		{name: "RSA key, PKCS #1", chain: rsaChain,
			key: pemPrivateKey(t, "RSA PRIVATE KEY", rsaKey)},
	}
	return pki
}

// files returns the files of an mTLS setting for the client certificate and key: the
// serverCa file holds an unrelated CA certificate and the server CA certificate, a rootCAs
// file the unrelated CA certificate.
func (p *testPKI) files(key clientKey) mtlsFiles {
	return mtlsFiles{
		cert:     key.chain,
		key:      key.key,
		serverCA: append(slices.Clone(p.otherCAPEM), p.serverCAPEM...),
		rootCA:   p.otherCAPEM,
	}
}

// serverTLSConfig returns the TLS configuration of a server with a certificate for
// serverName ("localhost" if empty) and 127.0.0.1, which requires a client certificate
// issued by the client root CA.
func (p *testPKI) serverTLSConfig(t *testing.T, serverName string) *tls.Config {
	t.Helper()
	if serverName == "" {
		serverName = "localhost"
	}
	key := ecdsaKey(t)
	cert := newCertificate(t, serverName, false, []string{serverName}, key,
		p.serverCA, p.serverCAKey)
	return &tls.Config{
		Certificates: []tls.Certificate{
			{Certificate: [][]byte{cert.Raw}, PrivateKey: key, Leaf: cert},
		},
		ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs:  p.clientRoots,
		MinVersion: tls.VersionTLS12,
	}
}

// handshake connects with the client configuration to a TLS server with a certificate for
// serverName (see serverTLSConfig), and returns the error of the client or the server.
func (p *testPKI) handshake(t *testing.T, client *tls.Config, serverName string) error {
	t.Helper()
	listener, err := tls.Listen("tcp", "127.0.0.1:0", p.serverTLSConfig(t, serverName))
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	serverErr := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverErr <- err
			return
		}
		defer conn.Close()
		tlsConn, _ := conn.(*tls.Conn)
		if err := tlsConn.HandshakeContext(t.Context()); err != nil {
			serverErr <- fmt.Errorf("server: %w", err)
			return
		}
		name := tlsConn.ConnectionState().PeerCertificates[0].Subject.CommonName
		if name != clientCommonName {
			serverErr <- fmt.Errorf("server: client certificate %q, want %q",
				name, clientCommonName)
			return
		}
		serverErr <- nil
	}()

	client = client.Clone()
	if client.ServerName == "" {
		client.ServerName = "localhost"
	}
	dialer := &tls.Dialer{Config: client}
	conn, err := dialer.DialContext(t.Context(), "tcp", listener.Addr().String())
	if err != nil {
		return fmt.Errorf("client: %w", err)
	}
	defer conn.Close()
	return <-serverErr
}

// newCertificate creates a certificate for key, issued by parent with parentKey, or
// self-signed if parent is nil: a CA certificate if ca is true, a server certificate if
// dnsNames are given, else a client certificate.
func newCertificate(t *testing.T, commonName string, ca bool, dnsNames []string, key crypto.Signer,
	parent *x509.Certificate, parentKey crypto.Signer,
) *x509.Certificate {
	t.Helper()
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 64))
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	switch {
	case ca:
		template.IsCA = true
		template.BasicConstraintsValid = true
		template.KeyUsage |= x509.KeyUsageCertSign
	case dnsNames != nil:
		template.DNSNames = dnsNames
		template.IPAddresses = []net.IP{net.IPv4(127, 0, 0, 1)}
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	default:
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	}
	if _, isRSA := key.(*rsa.PrivateKey); isRSA {
		template.KeyUsage |= x509.KeyUsageKeyEncipherment
	}
	if parent == nil {
		parent, parentKey = template, key
	}
	der, err := x509.CreateCertificate(rand.Reader, template, parent, key.Public(), parentKey)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return cert
}

func ecdsaKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func rsaTestKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func pemCertificate(cert *x509.Certificate) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
}

// pemPrivateKey encodes key in a PEM block of the given type: "PRIVATE KEY" (PKCS #8),
// "RSA PRIVATE KEY" (PKCS #1) or "EC PRIVATE KEY" (SEC 1).
func pemPrivateKey(t *testing.T, blockType string, key crypto.Signer) []byte {
	t.Helper()
	var der []byte
	var err error
	switch blockType {
	case "PRIVATE KEY":
		der, err = x509.MarshalPKCS8PrivateKey(key)
	case "RSA PRIVATE KEY":
		rsaKey, _ := key.(*rsa.PrivateKey)
		der = x509.MarshalPKCS1PrivateKey(rsaKey)
	case "EC PRIVATE KEY":
		ecKey, _ := key.(*ecdsa.PrivateKey)
		der, err = x509.MarshalECPrivateKey(ecKey)
	default:
		t.Fatalf("unknown block type %q", blockType)
	}
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der})
}
