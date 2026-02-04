package session_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openkcm/common-sdk/pkg/openid"

	"github.com/openkcm/session-manager/internal/session"
)

func StartOIDCServer(t *testing.T, fail bool, algs ...string) *httptest.Server {
	t.Helper()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error": "invalid_request", "error_description": "Token exchange failed"}`))
			return
		}
		// Determine supported algorithms by passed arguments or set to default value
		var algList []string
		if len(algs) == 0 {
			algList = []string{"RS256"}
		} else {
			algList = algs
		}

		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(openid.Configuration{
				Issuer:                           server.URL,
				AuthorizationEndpoint:            server.URL + "/oauth2/authorize",
				TokenEndpoint:                    server.URL + "/oauth2/token",
				JwksURI:                          server.URL + "/.well-known/jwks.json",
				IDTokenSigningAlgValuesSupported: algList,
			})
		case "/.well-known/jwks.json":
			_, _ = w.Write([]byte(`{"keys":[{"kty": "RSA", "e": "AQAB", "use": "sig", "kid": "MwK4iAYDIILiA_ymyjAwMGAaLlW84jOAqR0V-oojuIk", "alg": "RS256", "n": "nD89GVZMXuv_MSbH_SqDnU5oQgLlcH6yGe5LkXSdP_UzBXt49wPRoVHE-W981oylw9vhzfNBE8JY0PSkxVvYCWwYP86YWVtJix23iONYpXeAH9M1ep4Gzo1y0XnjAKURi-sN5T5nUBZ-fkODvyr6ALIUG3AXzaRow1RMmhUOx1spKGS34DJPv0D3E6aVcGkwgUwZcBhObYxGQdMAYi-OYDDS3uAkFciO3G1Bpz4nyW_JaV7i4zkMOH6-2wYFt1fjMsyc0lt1eRqdUVdANy0kDtmIXnjgjKN0Isr16flzfRDXfOQmaBPp14hQPiAgVFaqvTIvXucXOkiWcAQWhas2Aw"}]}`))
		case "/oauth2/token":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			tokenResponse := session.TokenResponse{
				AccessToken:  "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWUsImlhdCI6MTUxNjIzOTAyMn0.m3RtvePjRxhO0-O2FOzJvYWUNZqxdQ5p4wI5YTjC8BmFzCH-2gMjk1RSG00-_q0PkcKqoD_hZFVub88298nhF7WpEj2pEWvDhWeG4g3H4JxcFw-a2Pwam80qdrOOA8NkmDtTewC90yshYcGktGMHk5jjfh4sKRaZz9FmkBpc2G9I1NyxcCyj9yatMu64yFDNa0-CbaSsWyCFgsKvNxM944nJkT7q3OLFz_Tgn4HSXExEDE_Xkwhz6zykg12tcU9-5Fk40yEfOEaBCTmuv3AMguBOlEBD1X2IsPcz03My5bpECEFIuRbqu-xvny1vEhzjNB565uk4Es9PdLwi_6frWA",
				RefreshToken: "refresh-token",
				IDToken:      "eyJhbGciOiJSUzI1NiIsImtpZCI6Ik13SzRpQVlESUlMaUFfeW15akF3TUdBYUxsVzg0ak9BcVIwVi1vb2p1SWsiLCJ0eXAiOiJKV1QifQ.eyJzdWIiOiJqd3QtdGVzdCIsImp0aSI6IjIzNDE0MzUiLCJhdF9oYXNoIjoicVJiNThaanpTZFByYnpGQ3hielJFZyIsIm5iZiI6MTc2NDExNDM3MSwiZXhwIjoxNzY0MTI1MTcxLCJpYXQiOjE3NjQxMTQzNzEsImlzcyI6InNhcCIsImF1ZCI6Imh0dHBzOi8vZXhhbXBsZS5jb20ifQ.JhC2oGYRHTL4NVaz1CZKWop_Iq54fxQOQL2pJap1LIMFKRz9RqgZr_WMulBLjNxppS3v5KFaMMp28YirzhzJQVbIlrEuUQZQCeODmLYSVkyeQKGb9WTSirzZInZbICjfocgppSzZ_Z8_P0GSS_h4IEFgcK0jnfb-2O_Xef1dYSoxA-sOFCxvn48jnjBLNjRQh2uYY61unJRzAbchXTBCtTSKNL1SEM4rCvV9b9dfYKBSlaQ11DKzzC1Zd5xG4JNkrbDXYu6MAxYLz_getXsQh6rVqOnMjUOMQLjUcuMuSva1Fh9gCeJNWsy34bh6lfScBb67L3i5D1s8pciLYTNMDQ",
				TokenType:    "Bearer",
				ExpiresIn:    3600,
			}
			_ = json.NewEncoder(w).Encode(tokenResponse)
		}
	}))

	return server
}

func StartAuditServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success": true}`))
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
}
