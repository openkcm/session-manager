package session_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openkcm/session-manager/internal/oidc"
	"github.com/openkcm/session-manager/pkg/session"
)

func StartOIDCServer(t *testing.T, fail bool) *httptest.Server {
	t.Helper()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error": "invalid_request", "error_description": "Token exchange failed"}`))
			return
		}

		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(oidc.Configuration{
				Issuer:                server.URL,
				AuthorizationEndpoint: server.URL + "/oauth2/authorize",
				TokenEndpoint:         server.URL + "/oauth2/token",
				JwksURI:               server.URL + "/.well-known/jwks.json",
			})
		case "/.well-known/jwks.json":
			_, _ = w.Write([]byte(`{"keys":[{"kty": "RSA", "e": "AQAB", "use": "sig", "kid": "7cdrxOwDtBcW6ZmoW1CHjx2f74xqS6GAwJXOUd_oECw", "alg": "RS256", "n": "nMds_LftGh9YWfCuKfTU9rHezOPOUzooalZXIXMBnj4Xd7EQieVH4acwIlGQDsy9FasnSUzok4eeuJR1nmz7I5d0qIDjw_SItsFe83KetfFBLPsoCrR3kzcuof8KG3_N7pTGWMyl9cb8QTMzRYgzSrfgMJgi1TCHQq5uE-CWdjaCTklJgvnUb9QjYoyf3CkGz6hjlfu1TPw2CQfVXy0fW5jT9S6d10zYfYXnfeYxZFiKBgv2YNUPtwnejs0mZcE7lLyURf1tgkgZheHNde6Nz8UC0HEbGKBT6I-WXaFUJsmI5GDsQXTNfp6YmdYk_s-rM4bz-Hg51XI0JWk4J2bUyQ"}]}`))
		case "/oauth2/token":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			tokenResponse := session.TokenResponse{
				AccessToken:  "access-token",
				RefreshToken: "refresh-token",
				IDToken:      "eyJhbGciOiJSUzI1NiIsImtpZCI6IjdjZHJ4T3dEdEJjVzZabW9XMUNIangyZjc0eHFTNkdBd0pYT1VkX29FQ3ciLCJ0eXAiOiJKV1QifQ.eyJzdWIiOiJqd3QtdGVzdCIsImp0aSI6IjIzNDE0MzUiLCJuYmYiOjE3NjA1NzcwMjgsImV4cCI6MTc2MDU4NzgyOCwiaWF0IjoxNzYwNTc3MDI4LCJpc3MiOiJkYXJ3aW5MYWJzIiwiYXVkIjoiaHR0cDovL3d3dy5kYXJ3aW4tbGFicy5jb20ifQ.TbUiSRxNE-x2NYc_9CkLt59caV_CeOxaaHjbtBekWeKSnYXlZIOqf6qikdVhKwN3IdssUi5af6E2tVEvM4fAZuCGKy7qkHXqvitxm2XLfZPvQzscrN7L476rjUaEr2HcjqoOmhPwcgTfeJJRp9o_JIqvtb-NXhIZPbPBkinTWFIArLfcJ1WZx4fYbXY7nixunJfQqYYtZSP_OukzRbAK5qwPj55USPFhh3IBWrUsS4x_YOiF8PITldLhCYIFNmhI5vkT6KwaWVYAVZPnwLARSW0nZAKnv_qAuhwHbhP8Et746Qw-WF-5K2Ij3YlgsNG-6_c0ID2MwBhoqpg-1sFcug",
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
